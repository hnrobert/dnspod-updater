package updater

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hnrobert/dnspod-updater/internal/config"
	"github.com/hnrobert/dnspod-updater/internal/dnspod"
)

type IPDetector interface {
	DetectIPv4() (net.IP, string, error)
}

type DNSPodClient interface {
	RecordInfo(ctx context.Context, req dnspod.CommonRequest, recordID int) (dnspod.RecordInfoResponse, error)
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

	info, err := u.opt.DNSPod.RecordInfo(ctx, common, u.opt.Config.RecordID)
	if err != nil {
		return fmt.Errorf("Record.Info failed: %w", err)
	}
	current := strings.TrimSpace(info.Record.Value)
	u.opt.Logger.Printf("record current value=%q name=%q", current, info.Record.Name)

	if current == want {
		u.opt.Logger.Printf("no update needed (same IP)")
		return nil
	}

	var weightPtr *int
	if u.opt.Config.Weight >= 0 {
		w := u.opt.Config.Weight
		weightPtr = &w
	}

	_, err = u.opt.DNSPod.RecordModify(ctx, common, u.opt.Config.RecordID, dnspod.ModifyRecordParams{
		SubDomain:    u.opt.Config.SubDomain,
		RecordType:   u.opt.Config.RecordType,
		RecordLine:   u.opt.Config.RecordLine,
		RecordLineID: u.opt.Config.RecordLineID,
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
