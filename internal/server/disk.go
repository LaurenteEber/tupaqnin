package server

import (
	"context"
	"syscall"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// diskFullShare is the part of the disk that closes new projects (espec. §6).
const diskFullShare = 0.80

// DiskFull is the real check of espec. §6 over dir: true at 80 % of the
// disk or more, counted as df does (the blocks reserved for root are not
// room anyone here can use).
func DiskFull(dir string) func() (bool, error) {
	return func() (bool, error) {
		var fs syscall.Statfs_t
		if err := syscall.Statfs(dir, &fs); err != nil {
			return false, err
		}
		used := fs.Blocks - fs.Bfree
		room := used + fs.Bavail
		if room == 0 {
			return false, nil
		}
		return float64(used)/float64(room) >= diskFullShare, nil
	}
}

// SpendChecker is what the runner knows of the caps (B-89).
type SpendChecker interface {
	CreateBlocked(ctx context.Context, invitationID string) (string, error)
	StoppedBy(ctx context.Context, projectID string) (string, error)
}

// createBlocked is the code that refuses a new project for the caps, or "".
// [EL DISEÑO NO LO DICE]: these come before account_required and link_full;
// and a disk that cannot be read does not close the door, it is logged.
func (s *Server) createBlocked(ctx context.Context, sess store.Session) (string, error) {
	if c, ok := s.starter.(SpendChecker); ok {
		code, err := c.CreateBlocked(ctx, sess.InvitationID)
		if err != nil || code != "" {
			return code, err
		}
	}
	if s.diskFull == nil {
		return "", nil
	}
	full, err := s.diskFull()
	if err != nil {
		s.logger.Error("disk check failed", "error", err.Error())
		return "", nil
	}
	if !full {
		return "", nil
	}
	link, err := s.db.LinkSpendState(ctx, sess.InvitationID, s.now())
	if err != nil {
		return "", err
	}
	if link.Label == store.LabelJury {
		return "", nil
	}
	return "disk_full", nil
}
