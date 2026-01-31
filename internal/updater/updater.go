package updater

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hnrobert/dnspod-updater/internal/config"
	"github.com/hnrobert/dnspod-updater/internal/dnspod"
	"github.com/hnrobert/dnspod-updater/internal/ipdetect"
)

type IPDetector interface {
	DetectIPv4() (net.IP, string, error)
}

type DNSPodClient interface {
	RecordInfo(ctx context.Context, req dnspod.CommonRequest, recordID int) (dnspod.RecordInfoResponse, error)
	RecordList(ctx context.Context, req dnspod.CommonRequest, p dnspod.RecordListParams) (dnspod.RecordListResponse, error)
	RecordModify(ctx context.Context, req dnspod.CommonRequest, recordID int, p dnspod.ModifyRecordParams) (dnspod.RecordModifyResponse, error)
}

type Options struct {
	Config     config.Config
	Detector   IPDetector
	DNSPod     DNSPodClient
	Logger     *log.Logger
	StartDelay time.Duration
}

type Updater struct {
	opt Options
}

func New(opt Options) *Updater {
	if opt.Logger == nil {
		opt.Logger = log.Default()
	}
	return &Updater{opt: opt}
}

func (u *Updater) Run(ctx context.Context) error {
	if u.opt.StartDelay > 0 {
		u.opt.Logger.Printf("start delay: %s", u.opt.StartDelay)
		t := time.NewTimer(u.opt.StartDelay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}

	// Always run once on startup.
	if err := u.checkAndUpdateOnce(ctx); err != nil {
		u.opt.Logger.Printf("startup check failed: %v", err)
	}

	if u.opt.Config.OneShot || u.opt.Config.CheckInterval <= 0 {
		u.opt.Logger.Printf("exit after startup (ONESHOT=%v, CHECK_INTERVAL=%s)", u.opt.Config.OneShot, u.opt.Config.CheckInterval)
		return nil
	}

	ticker := time.NewTicker(u.opt.Config.CheckInterval)
	defer ticker.Stop()

	u.opt.Logger.Printf("watching IP changes every %s", u.opt.Config.CheckInterval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := u.checkAndUpdateOnce(ctx); err != nil {
				u.opt.Logger.Printf("periodic check failed: %v", err)
			}
		}
	}
}

func (u *Updater) checkAndUpdateOnce(ctx context.Context) error {
	ip, src, err := u.opt.Detector.DetectIPv4()
	if err != nil {
		if errors.Is(err, ipdetect.ErrWiFiSSIDNotMatched) || errors.Is(err, ipdetect.ErrWiFiSSIDUnavailable) {
			u.opt.Logger.Printf("wifi ssid constraint not satisfied, skip: %v", err)
			return nil
		}
		return fmt.Errorf("detect ip: %w", err)
	}
	want := ip.String()
	u.opt.Logger.Printf("detected IPv4=%s via %s", want, src)

	common := dnspod.CommonRequest{
		LoginToken:   u.opt.Config.LoginToken,
		Format:       u.opt.Config.Format,
		Lang:         u.opt.Config.Lang,
		ErrorOnEmpty: u.opt.Config.ErrorOnEmpty,
		Domain:       u.opt.Config.Domain,
		DomainID:     u.opt.Config.DomainID,
	}

	recordID := u.opt.Config.RecordID
	var current string
	var recordName string
	var recordType string
	var recordLine string
	var recordLineID string

	if recordID > 0 {
		info, err := u.opt.DNSPod.RecordInfo(ctx, common, recordID)
		if err != nil {
			return fmt.Errorf("Record.Info failed: %w", err)
		}
		current = strings.TrimSpace(info.Record.Value)
		recordName = strings.TrimSpace(info.Record.Name)
		recordType = strings.TrimSpace(info.Record.Type)
		recordLine = strings.TrimSpace(info.Record.Line)
		recordLineID = strings.TrimSpace(info.Record.LineID)
		u.opt.Logger.Printf("target record id=%d name=%q value=%q", recordID, recordName, current)
	} else {
		// Resolve record id by (domain + sub_domain). Pick the first matching record.
		list, err := u.opt.DNSPod.RecordList(ctx, common, dnspod.RecordListParams{
			SubDomain:  u.opt.Config.SubDomain,
			RecordType: u.opt.Config.RecordType,
			Offset:     0,
			Length:     100,
		})
		if err != nil {
			return fmt.Errorf("Record.List failed: %w", err)
		}
		if len(list.Records) == 0 {
			return fmt.Errorf("no records found for sub_domain=%q", u.opt.Config.SubDomain)
		}

		// Prefer exact type match (e.g. A). Otherwise just take the first record.
		idx := 0
		wantType := strings.ToUpper(strings.TrimSpace(u.opt.Config.RecordType))
		if wantType != "" {
			for i := range list.Records {
				if strings.ToUpper(strings.TrimSpace(list.Records[i].Type)) == wantType {
					idx = i
					break
				}
			}
		}

		rec := list.Records[idx]
		parsedID, err := strconv.Atoi(strings.TrimSpace(rec.ID))
		if err != nil || parsedID <= 0 {
			return fmt.Errorf("invalid record id from Record.List: %q", rec.ID)
		}
		recordID = parsedID
		current = strings.TrimSpace(rec.Value)
		recordName = strings.TrimSpace(rec.Name)
		recordType = strings.TrimSpace(rec.Type)
		recordLine = strings.TrimSpace(rec.Line)
		recordLineID = strings.TrimSpace(rec.LineID)
		u.opt.Logger.Printf("resolved record id=%d name=%q type=%q line_id=%q value=%q", recordID, recordName, recordType, recordLineID, current)
	}

	if current == want {
		u.opt.Logger.Printf("no update needed (same IP)")
		return nil
	}

	var weightPtr *int
	if u.opt.Config.Weight >= 0 {
		w := u.opt.Config.Weight
		weightPtr = &w
	}

	// Preserve the existing record type/line by default.
	// If user provided DNSPOD_RECORD_LINE_ID, it takes precedence.
	useLineID := strings.TrimSpace(u.opt.Config.RecordLineID)
	useLine := u.opt.Config.RecordLine
	if useLineID == "" {
		useLineID = recordLineID
		useLine = recordLine
	}

	useType := u.opt.Config.RecordType
	if strings.TrimSpace(useType) == "" {
		useType = recordType
	}

	_, err = u.opt.DNSPod.RecordModify(ctx, common, recordID, dnspod.ModifyRecordParams{
		SubDomain:    recordName,
		RecordType:   useType,
		RecordLine:   useLine,
		RecordLineID: useLineID,
		Value:        want,
		MX:           u.opt.Config.MX,
		TTL:          u.opt.Config.TTL,
		Status:       u.opt.Config.Status,
		Weight:       weightPtr,
	})
	if err != nil {
		return fmt.Errorf("Record.Modify failed: %w", err)
	}

	u.opt.Logger.Printf("updated record to %s", want)
	return nil
}
