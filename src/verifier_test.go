package src

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/prysmaticlabs/prysm/v5/api/server/structs"
	"github.com/stretchr/testify/assert"
)

func Test_BuildSyncCommitteeUpdate_Mainnet_ChainSafe(t *testing.T) {
	start := 290
	end := 1420

	firstIsBootstrap := start == 290 //
	saveFile := false

	updates := make([]*structs.LightClientUpdateResponse, 0)

	if firstIsBootstrap {
		var bootstrap structs.LightClientBootstrap
		fn := fmt.Sprintf("../mainnet/updates/bootstrap_mainnet.chainsafe")
		data, err := os.ReadFile(fn)
		assert.NoError(t, err)
		err = json.Unmarshal(data, &bootstrap)
		assert.NoError(t, err)

		update := structs.LightClientUpdateResponse{
			Version: Altair,
			Data: &structs.LightClientUpdate{
				NextSyncCommittee: bootstrap.CurrentSyncCommittee,
			},
		}
		updates = append(updates, &update)
	}

	for i := start; i <= end; i++ {
		fn := fmt.Sprintf("../mainnet/updates/update_mainnet_%v.chainsafe", i)
		data, err := os.ReadFile(fn)
		assert.NoError(t, err)

		var update structs.LightClientUpdateResponse
		err = json.Unmarshal(data, &update)
		assert.NoError(t, err)
		updates = append(updates, &update)
	}

	for i := 0; i < len(updates)-1; i++ {
		var scUpdate SyncCommitteeUpdate
		_updates := [2]*structs.LightClientUpdateResponse{updates[i], updates[i+1]}
		err := scUpdate.FromLightClientUpdateResponse(_updates)
		assert.NoError(t, err)

		valid, err := scUpdate.Verify()
		assert.NoError(t, err)
		assert.Equal(t, true, valid)

		slot, ok := big.NewInt(0).SetString(scUpdate.AttestedHeader.Slot, 10)
		if !ok {
			panic("failed to parse slot")
		}
		period := slot.Int64() / 8192
		fmt.Printf("update_%v verify pass\n", period)

		if saveFile {
			dir := fmt.Sprintf("../mainnet/sync-committee/unit/sc%v", period)
			err = os.MkdirAll(dir, 0o777)
			assert.NoError(t, err)

			fn := fmt.Sprintf("%v/mainnet_sync_committee_update_%v.json", dir, period)
			data, err := json.Marshal(scUpdate)
			assert.NoError(t, err)

			err = os.WriteFile(fn, data, 0o777)
			assert.NoError(t, err)
		}
	}
}
