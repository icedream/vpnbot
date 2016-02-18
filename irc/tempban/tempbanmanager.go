package tempban

import (
	"encoding/gob"
	"errors"
	"io"

	"github.com/fluffle/goirc/logging"
)

var currentExportVersion = uint64(0x0)

type TemporaryBanExport struct {
	Bans []TemporaryBan
}

type TemporaryBanManager struct {
	data *TemporaryBanExport

	banRemovalTrigger map[TemporaryBan]chan interface{}
	BanExpiredFunc    func(TemporaryBan)

	DisableExpiry bool
}

func (tbmgr *TemporaryBanManager) onBanExpired(ban TemporaryBan) {
	f := tbmgr.BanExpiredFunc
	if f != nil {
		f(ban)
	}
}

func (tbmgr *TemporaryBanManager) GetAll() []TemporaryBan {
	return tbmgr.data.Bans
}

func (tbmgr *TemporaryBanManager) Get(hostmask string) (foundBan TemporaryBan, ok bool) {
	for _, ban := range tbmgr.data.Bans {
		if ban.Hostmask == hostmask {
			foundBan = ban
			ok = true
			return
		}
	}
	return
}

func (tbmgr *TemporaryBanManager) Add(ban TemporaryBan) error {
	if _, ok := tbmgr.Get(ban.Hostmask); ok {
		return ErrHostmaskAlreadyBanned
	}
	tbmgr.data.Bans = append(tbmgr.data.Bans, ban)
	if !tbmgr.DisableExpiry {
		tbmgr.handleBan(&ban)
	}
	return nil
}

func (tbmgr *TemporaryBanManager) handleBan(banPtr *TemporaryBan) {
	ban := *banPtr
	banRemovalTriggerChan := make(chan interface{})
	tbmgr.banRemovalTrigger[ban] = banRemovalTriggerChan
	go func() {
		select {
		case <-ban.WaitUntilExpired(): // expired
			close(banRemovalTriggerChan)
			for index, foundBan := range tbmgr.data.Bans {
				if ban == foundBan {
					tbmgr.data.Bans = append(tbmgr.data.Bans[0:index], tbmgr.data.Bans[index+1:]...)
					break
				}
			}
			tbmgr.onBanExpired(ban)

		case _, _ = <-banRemovalTriggerChan: // manually removed
		}
		delete(tbmgr.banRemovalTrigger, ban)
	}()
}

func (tbmgr *TemporaryBanManager) Remove(hostmask string) (foundBan TemporaryBan, deleted bool) {
	if _, ok := tbmgr.Get(hostmask); ok {
		for index, ban := range tbmgr.data.Bans {
			if ban.Hostmask == hostmask {
				close(tbmgr.banRemovalTrigger[ban])
				tbmgr.data.Bans = append(tbmgr.data.Bans[0:index], tbmgr.data.Bans[index+1:]...)
				foundBan = ban
				deleted = true
				return
			}
		}
	}
	return
}

func NewTemporaryBanManager() *TemporaryBanManager {
	return &TemporaryBanManager{
		data:              new(TemporaryBanExport),
		banRemovalTrigger: make(map[TemporaryBan]chan interface{}),
	}
}

func (tbmgr *TemporaryBanManager) Import(r io.Reader) error {
	decoder := gob.NewDecoder(r)

	var exportVersion uint64
	if err := decoder.Decode(&exportVersion); err != nil {
		return err
	}

	switch exportVersion {
	case 0x0:
		// Structure:
		// - Bans = []*TemporaryBan
		if err := decoder.Decode(tbmgr.data); err != nil {
			return err
		}
	default:
		return errors.New("Invalid version found in input data")
	}

	// Run background handler for each temporary ban
	if !tbmgr.DisableExpiry {
		for _, ban := range tbmgr.data.Bans {
			tbmgr.handleBan(&ban)
		}
	}

	logging.Info("Imported %v bans.", len(tbmgr.data.Bans))

	return nil
}

func (tbmgr *TemporaryBanManager) Export(w io.Writer) error {
	encoder := gob.NewEncoder(w)

	if err := encoder.Encode(currentExportVersion); err != nil {
		return err
	}

	// Structure:
	// - Bans = []*TemporaryBan
	if err := encoder.Encode(tbmgr.data); err != nil {
		return err
	}

	return nil
}
