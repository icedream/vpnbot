package tempban

import (
	"encoding/gob"
	"errors"
	"io"

	"github.com/icedream/vpnbot/debug/sync"

	"github.com/fluffle/goirc/logging"
)

var currentExportVersion = uint64(0x0)

type TemporaryBanExport struct {
	Bans []TemporaryBan
}

type TemporaryBanManager struct {
	data *TemporaryBanExport

	dataSync *sync.RWMutex

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
	tbmgr.dataSync.RLock()
	defer tbmgr.dataSync.RUnlock()

	return tbmgr.data.Bans
}

func (tbmgr *TemporaryBanManager) Get(hostmask string) (TemporaryBan, bool) {
	tbmgr.dataSync.RLock()
	defer tbmgr.dataSync.RUnlock()

	return tbmgr.getNoLock(hostmask)
}

func (tbmgr *TemporaryBanManager) getNoLock(hostmask string) (foundBan TemporaryBan, ok bool) {
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
	tbmgr.dataSync.Lock()
	defer tbmgr.dataSync.Unlock()

	if _, ok := tbmgr.getNoLock(ban.Hostmask); ok {
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
			ban, _ := tbmgr.Remove(ban.Hostmask)
			tbmgr.onBanExpired(ban)

		case _, _ = <-banRemovalTriggerChan: // manually removed
		}
		delete(tbmgr.banRemovalTrigger, ban)
	}()
}

func (tbmgr *TemporaryBanManager) Remove(hostmask string) (foundBan TemporaryBan, deleted bool) {
	tbmgr.dataSync.Lock()
	defer tbmgr.dataSync.Unlock()

	for index, ban := range tbmgr.data.Bans {
		if ban.Hostmask == hostmask {
			close(tbmgr.banRemovalTrigger[ban])
			tbmgr.data.Bans = append(tbmgr.data.Bans[0:index], tbmgr.data.Bans[index+1:]...)
			foundBan = ban
			deleted = true
			return
		}
	}
	return
}

func NewTemporaryBanManager() *TemporaryBanManager {
	return &TemporaryBanManager{
		data:              new(TemporaryBanExport),
		dataSync:          &sync.RWMutex{},
		banRemovalTrigger: make(map[TemporaryBan]chan interface{}),
	}
}

func (tbmgr *TemporaryBanManager) Import(r io.Reader) error {
	tbmgr.dataSync.Lock()
	defer tbmgr.dataSync.Unlock()

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
	tbmgr.dataSync.RLock()
	defer tbmgr.dataSync.RUnlock()

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
