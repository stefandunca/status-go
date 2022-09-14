package wallet

import (
	"database/sql"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/status-im/status-go/appdatabase"
	"github.com/status-im/status-go/sqlite"
)

func setupTestSavedAddressesDB(t *testing.T) (*SavedAddressesManager, func()) {
	tmpfile, err := ioutil.TempFile("", "wallet-saved_addresses-tests-")
	require.NoError(t, err)
	db, err := appdatabase.InitializeDB(tmpfile.Name(), "wallet-saved_addresses-tests", sqlite.ReducedKDFIterationsNumber)
	require.NoError(t, err)
	return &SavedAddressesManager{db}, func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.Remove(tmpfile.Name()))
	}
}

func TestSavedAddresses(t *testing.T) {
	manager, stop := setupTestSavedAddressesDB(t)
	defer stop()

	rst, err := manager.GetSavedAddressesForChainID(777)
	require.NoError(t, err)
	require.Nil(t, rst)

	sa := SavedAddress{
		Address:   common.Address{1},
		Name:      "Zilliqa",
		Favourite: true,
		ChainID:   777,
	}

	_, err = manager.UpsertSavedAddress(sa)
	require.NoError(t, err)

	rst, err = manager.GetSavedAddressesForChainID(777)
	require.NoError(t, err)
	require.Equal(t, 1, len(rst))
	require.Equal(t, sa, rst[0])

	_, err = manager.DeleteSavedAddress(777, sa.Address)
	require.NoError(t, err)

	rst, err = manager.GetSavedAddressesForChainID(777)
	require.NoError(t, err)
	require.Equal(t, 0, len(rst))
}

func contains[T comparable](container []T, element T) bool {
	for _, e := range container {
		if e == element {
			return true
		}
	}
	return false
}

func haveSameElements[T comparable](a []T, b []T) bool {
	for _, v := range a {
		if !contains(b, v) {
			return false
		}
	}
	return true
}

func TestSavedAddressesMetadata(t *testing.T) {
	manager, stop := setupTestSavedAddressesDB(t)
	defer stop()

	savedAddresses, metadatas, err := manager.GetRawSavedAddresses()
	require.NoError(t, err)
	require.Nil(t, savedAddresses)
	require.Nil(t, metadatas)

	// Add raw saved addresses
	sa1 := SavedAddress{
		Address:   common.Address{1},
		ChainID:   777,
		Name:      "Raw",
		Favourite: true,
	}

	meta1 := SavedAddressMeta{
		Removed:     false,
		SyncClock:   sql.NullInt64{0, false},
		UpdateClock: sql.NullInt64{234, true},
	}

	err = manager.addRawSavedAddress(sa1, meta1, nil)
	require.NoError(t, err)

	dbSavedAddresses, dbMetadatas, err := manager.GetRawSavedAddresses()
	require.NoError(t, err)
	require.Equal(t, 1, len(dbSavedAddresses))
	require.Equal(t, 1, len(dbMetadatas))
	require.Equal(t, sa1, dbSavedAddresses[0])
	require.Equal(t, meta1, dbMetadatas[0])

	// Add simple saved address without sync metadata
	sa2 := SavedAddress{
		Address:   common.Address{2},
		ChainID:   777,
		Name:      "Simple",
		Favourite: false,
	}

	_, err = manager.UpsertSavedAddress(sa2)
	require.NoError(t, err)

	dbSavedAddresses, dbMetadatas, err = manager.GetRawSavedAddresses()
	require.NoError(t, err)
	require.Equal(t, 2, len(dbSavedAddresses))
	require.Equal(t, 2, len(dbMetadatas))
	// The order is not guaranteed check raw entry to decide
	rawIndex := 0
	simpleIndex := 1
	if dbSavedAddresses[0] != sa1 {
		rawIndex = 1
		simpleIndex = 0
	}
	require.Equal(t, sa1, dbSavedAddresses[rawIndex])
	require.Equal(t, sa2, dbSavedAddresses[simpleIndex])
	require.Equal(t, meta1, dbMetadatas[rawIndex])
	// Check the default values
	require.False(t, dbMetadatas[simpleIndex].Removed)
	require.False(t, dbMetadatas[simpleIndex].SyncClock.Valid)
	require.True(t, dbMetadatas[simpleIndex].UpdateClock.Valid)
	require.Greater(t, dbMetadatas[simpleIndex].UpdateClock.Int64, int64(0))

	sa2Older := sa2
	sa2Older.Name = "Conditional, NOT updated"
	sa2Older.Favourite = true

	sa2Newer := sa2
	sa2Newer.Name = "Conditional, updated"
	sa2Newer.Favourite = false

	// Try to add an older entry
	updated := false
	updated, err = manager.AddSavedAddressIfNewerUpdate(sa2Older, 10, dbMetadatas[simpleIndex].UpdateClock.Int64-1)
	require.NoError(t, err)
	require.False(t, updated)

	dbSavedAddresses, dbMetadatas, err = manager.GetRawSavedAddresses()
	require.NoError(t, err)

	rawIndex = 0
	simpleIndex = 1
	if dbSavedAddresses[0] != sa1 {
		rawIndex = 1
		simpleIndex = 0
	}

	require.Equal(t, 2, len(dbSavedAddresses))
	require.Equal(t, 2, len(dbMetadatas))
	require.True(t, haveSameElements([]SavedAddress{sa1, sa2}, dbSavedAddresses))
	require.Equal(t, meta1, dbMetadatas[rawIndex])

	// Try to update sa2 with a newer entry
	updatedClock := dbMetadatas[simpleIndex].UpdateClock.Int64 + 1
	updated, err = manager.AddSavedAddressIfNewerUpdate(sa2Newer, 11, updatedClock)
	require.NoError(t, err)
	require.True(t, updated)

	dbSavedAddresses, dbMetadatas, err = manager.GetRawSavedAddresses()
	require.NoError(t, err)

	simpleIndex = 1
	if dbSavedAddresses[0] != sa1 {
		simpleIndex = 0
	}

	require.Equal(t, 2, len(dbSavedAddresses))
	require.Equal(t, 2, len(dbMetadatas))
	require.True(t, haveSameElements([]SavedAddress{sa1, sa2Newer}, dbSavedAddresses))
	require.Equal(t, updatedClock, dbMetadatas[simpleIndex].UpdateClock.Int64)

	// Try to delete the sa2 newer entry
	updatedDeleteClock := updatedClock + 1
	updated, err = manager.DeleteSavedAddressIfNewerUpdate(sa2Newer.ChainID, sa2Newer.Address, 12, updatedDeleteClock)
	require.NoError(t, err)
	require.True(t, updated)

	dbSavedAddresses, dbMetadatas, err = manager.GetRawSavedAddresses()
	require.NoError(t, err)

	simpleIndex = 1
	if dbSavedAddresses[0] != sa1 {
		simpleIndex = 0
	}

	require.Equal(t, 2, len(dbSavedAddresses))
	require.Equal(t, 2, len(dbMetadatas))
	require.True(t, dbMetadatas[simpleIndex].Removed)

	// Check that deleted entry is not returned with the regular API (non-raw)
	dbSavedAddresses, err = manager.GetSavedAddresses()
	require.NoError(t, err)
	require.Equal(t, 1, len(dbSavedAddresses))
}

func TestSavedAddressesCleanSoftDeletes(t *testing.T) {
	manager, stop := setupTestSavedAddressesDB(t)
	defer stop()

	firstTimestamp := 10
	for i := 0; i < 5; i++ {
		sa := SavedAddress{
			Address:   common.Address{byte(i)},
			ChainID:   777,
			Name:      "Test" + strconv.Itoa(i),
			Favourite: false,
		}

		meta := SavedAddressMeta{
			Removed:     true,
			SyncClock:   sql.NullInt64{Int64: 0, Valid: false},
			UpdateClock: sql.NullInt64{Int64: int64(firstTimestamp + i), Valid: true},
		}

		err := manager.addRawSavedAddress(sa, meta, nil)
		require.NoError(t, err)
	}

	err := manager.DeleteSoftRemovedSavedAddresses(int64(firstTimestamp + 3))
	require.NoError(t, err)

	dbSavedAddresses, dbMetadatas, err := manager.GetRawSavedAddresses()
	require.NoError(t, err)
	require.Equal(t, 2, len(dbSavedAddresses))
	require.True(t, haveSameElements([]int64{dbMetadatas[0].UpdateClock.Int64, dbMetadatas[1].UpdateClock.Int64}, []int64{int64(firstTimestamp + 3), int64(firstTimestamp + 4)}))
}
