package wallet

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var syncClockCreatedEditedHere = sql.NullInt64{Int64: 0, Valid: false}

// TODO: make them private?
type SavedAddressMeta struct {
	Removed     bool
	SyncClock   sql.NullInt64 // clock of the last sync
	UpdateClock sql.NullInt64 // wall clock used to deconflict concurrent updates
}

type SavedAddress struct {
	Address common.Address `json:"address"`
	// TODO: Add Emoji and Networks
	// Emoji    string         `json:"emoji"`
	Name      string `json:"name"`
	Favourite bool   `json:"favourite"`
	ChainID   uint64 `json:"chainId"`
	SavedAddressMeta
}

type SavedAddressesManager struct {
	db *sql.DB
}

func NewSavedAddressesManager(db *sql.DB) *SavedAddressesManager {
	return &SavedAddressesManager{db: db}
}

const rawQueryColumnsOrder = "address, name, favourite, network_id, removed, sync_clock, update_clock"

// Retrieve raw data based on SELECT query using rawQueryColumnsOrder
func getRawSavedAddressesFromDBRows(rows *sql.Rows) ([]SavedAddress, error) {
	var addresses []SavedAddress
	for rows.Next() {
		sa := SavedAddress{}
		// based on rawQueryColumnsOrder
		err := rows.Scan(&sa.Address, &sa.Name, &sa.Favourite, &sa.ChainID, &sa.Removed, &sa.SyncClock, &sa.UpdateClock)
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, sa)
	}

	return addresses, nil
}

func (sam *SavedAddressesManager) GetSavedAddressesForChainID(chainID uint64) ([]SavedAddress, error) {
	rows, err := sam.db.Query(fmt.Sprintf("SELECT %s FROM saved_addresses WHERE network_id = ? AND removed != 1", rawQueryColumnsOrder), chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addresses, err := getRawSavedAddressesFromDBRows(rows)
	return addresses, err
}

func (sam *SavedAddressesManager) GetSavedAddresses() ([]SavedAddress, error) {
	rows, err := sam.db.Query(fmt.Sprintf("SELECT %s FROM saved_addresses WHERE removed != 1", rawQueryColumnsOrder))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addresses, err := getRawSavedAddressesFromDBRows(rows)
	return addresses, err
}

// Provide access to the soft-delete and sync metadata
func (sam *SavedAddressesManager) GetRawSavedAddresses() ([]SavedAddress, error) {
	rows, err := sam.db.Query(fmt.Sprintf("SELECT %s FROM saved_addresses", rawQueryColumnsOrder))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return getRawSavedAddressesFromDBRows(rows)
}

func (sam *SavedAddressesManager) addRawSavedAddress(sa SavedAddress, tx *sql.Tx) error {
	sqlStatement := "INSERT OR REPLACE INTO saved_addresses (network_id, address, name, favourite, removed, sync_clock, update_clock) VALUES (?, ?, ?, ?, ?, ?, ?)"
	var err error
	var insert *sql.Stmt
	if tx != nil {
		insert, err = tx.Prepare(sqlStatement)
	} else {
		insert, err = sam.db.Prepare(sqlStatement)
	}
	if err != nil {
		return err
	}
	defer insert.Close()
	_, err = insert.Exec(sa.ChainID, sa.Address, sa.Name, sa.Favourite, sa.Removed, sa.SyncClock, sa.UpdateClock)
	return err
}

func (sam *SavedAddressesManager) UpsertSavedAddress(sa SavedAddress) (updatedClock int64, err error) {
	updateClock := time.Now().Unix()
	sa.UpdateClock = sql.NullInt64{Int64: updateClock, Valid: true}
	sa.SyncClock = syncClockCreatedEditedHere
	err = sam.addRawSavedAddress(sa, nil)
	if err != nil {
		return 0, err
	}
	return updateClock, nil
}

func (sam *SavedAddressesManager) startTransactionAndCheckIfNewerChange(ChainID uint64, Address common.Address, updateClock int64) (newer bool, tx *sql.Tx, err error) {
	tx, err = sam.db.Begin()
	if err != nil {
		return false, nil, err
	}
	row := tx.QueryRow("SELECT update_clock FROM saved_addresses WHERE network_id = ? AND address = ?", ChainID, Address)
	if err != nil {
		return false, tx, err
	}

	var dbUpdateClock int64
	err = row.Scan(&dbUpdateClock)
	if err != nil {
		return err == sql.ErrNoRows, tx, err
	}
	return dbUpdateClock <= updateClock, tx, nil
}

func (sam *SavedAddressesManager) AddSavedAddressIfNewerUpdate(sa SavedAddress, syncClock int64, updateClock int64) (insertedOrUpdated bool, err error) {
	newer, tx, err := sam.startTransactionAndCheckIfNewerChange(sa.ChainID, sa.Address, updateClock)
	defer func() {
		if err == nil {
			err = tx.Commit()
			return
		}
		_ = tx.Rollback()
	}()
	if !newer {
		return false, err
	}

	sa.SyncClock = sql.NullInt64{Int64: syncClock, Valid: true}
	sa.UpdateClock = sql.NullInt64{Int64: updateClock, Valid: true}
	err = sam.addRawSavedAddress(sa, tx)
	if err != nil {
		return false, err
	}

	return true, err
}

func (sam *SavedAddressesManager) DeleteSavedAddressIfNewerUpdate(chainID uint64, address common.Address, syncClock int64, updateClock int64) (deleted bool, err error) {
	newer, tx, err := sam.startTransactionAndCheckIfNewerChange(chainID, address, updateClock)
	defer func() {
		if err == nil {
			err = tx.Commit()
			return
		}
		_ = tx.Rollback()
	}()
	if !newer {
		return false, err
	}

	var insert *sql.Stmt
	insert, err = tx.Prepare(`INSERT OR REPLACE INTO saved_addresses (network_id, address, name, favourite, removed, sync_clock, update_clock) VALUES (?, ?, "", 0, 1, ?, ?)`)
	if err != nil {
		return false, err
	}
	defer insert.Close()
	_, err = insert.Exec(chainID, address, syncClock, updateClock)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (sam *SavedAddressesManager) DeleteSavedAddress(chainID uint64, address common.Address) (updatedClock int64, err error) {
	insert, err := sam.db.Prepare(`INSERT OR REPLACE INTO saved_addresses (network_id, address, name, favourite, removed, sync_clock, update_clock) VALUES (?, ?, "", 0, 1, NULL, ?)`)
	if err != nil {
		return 0, err
	}
	defer insert.Close()
	updateClock := time.Now().Unix()
	_, err = insert.Exec(chainID, address, updateClock)
	if err != nil {
		return 0, err
	}
	return updateClock, nil
}

func (sam *SavedAddressesManager) DeleteSoftRemovedSavedAddresses(threshold int64) error {
	_, err := sam.db.Exec(`DELETE FROM saved_addresses WHERE removed = 1 AND update_clock < ?`, threshold)
	return err
}
