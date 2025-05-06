package src

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/v5/api/server/structs"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	"github.com/prysmaticlabs/prysm/v5/container/trie"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
)

type SyncCommitteeUpdate struct {
	Version                 string                     `json:"version"`
	AttestedHeader          *structs.BeaconBlockHeader `json:"attested_header"`
	CurrentSyncCommittee    *structs.SyncCommittee     `json:"current_sync_committee,omitempty"`     //current_sync_committee
	SyncAggregate           *structs.SyncAggregate     `json:"sync_aggregate"`                       //sync_aggregate for attested_header, signed by current_sync_committee
	FinalizedHeader         *structs.BeaconBlockHeader `json:"finalized_header,omitempty"`           //finalized_header in attested_header.state_root
	FinalityBranch          []string                   `json:"finality_branch,omitempty"`            // finality_branch in attested_header.state_root
	NextSyncCommittee       *structs.SyncCommittee     `json:"next_sync_committee,omitempty"`        //next_sync_committee in finalized_header.state_root
	NextSyncCommitteeBranch []string                   `json:"next_sync_committee_branch,omitempty"` //next_sync_committee branch in finalized_header.state_root
	SignatureSlot           string                     `json:"signature_slot"`
}

// SyncCommitteeUpdate := Previous_LightClientUpdateResponse.NextSyncCommittee + Current_LightClientUpdateResponse
func (scu *SyncCommitteeUpdate) FromLightClientUpdateResponse(updates [2]*structs.LightClientUpdateResponse) error {
	switch updates[1].Version {
	case Altair, Bellatrix, Capella, Deneb:
		{
			var attestedHeader structs.LightClientHeader
			err := json.Unmarshal(updates[1].Data.AttestedHeader, &attestedHeader)
			if err != nil {
				return err
			}

			scu.AttestedHeader = attestedHeader.Beacon
			var finalizedHeader structs.LightClientHeader
			err = json.Unmarshal(updates[1].Data.FinalizedHeader, &finalizedHeader)
			if err != nil {
				return err
			}

			scu.FinalizedHeader = finalizedHeader.Beacon
		}
	case Electra:
		{
			var attestedHeader structs.LightClientHeaderDeneb
			err := json.Unmarshal(updates[1].Data.AttestedHeader, &attestedHeader)
			if err != nil {
				return err
			}
			scu.AttestedHeader = attestedHeader.Beacon

			var finalizedHeader structs.LightClientHeaderDeneb
			err = json.Unmarshal(updates[1].Data.FinalizedHeader, &finalizedHeader)
			if err != nil {
				return err
			}
			scu.FinalizedHeader = finalizedHeader.Beacon
		}
	default:
		return fmt.Errorf("unsupported version: %s", updates[1].Version)
	}

	currentSyncCommittee := updates[0].Data.NextSyncCommittee
	syncAggregate := updates[1].Data.SyncAggregate
	nextSyncCommittee := updates[1].Data.NextSyncCommittee

	scu.Version = updates[1].Version
	scu.CurrentSyncCommittee = currentSyncCommittee
	scu.SyncAggregate = syncAggregate
	scu.NextSyncCommittee = nextSyncCommittee
	scu.NextSyncCommitteeBranch = updates[1].Data.NextSyncCommitteeBranch
	scu.FinalityBranch = updates[1].Data.FinalityBranch
	scu.SignatureSlot = updates[1].Data.SignatureSlot

	valid, err := scu.Verify()
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("verifyLightClientUpdateInfo failed")
	}
	return nil
}

// https://github.com/ethereum/consensus-specs/blob/dev/specs/altair/light-client/sync-protocol.md#lightclientbootstrap
func (scu *SyncCommitteeUpdate) Verify() (bool, error) {
	domain, err := GetMainnetDomain(scu.Version)
	if err != nil {
		return false, err
	}

	nextSyncCommitteeIndex := NextSyncCommitteeIndex + 1<<NextSyncCommitteeDepth
	finalizedHeaderIndex := FinalizedHeaderIndex + 1<<FinalizedHeaderDepth

	if scu.Version == Electra {
		nextSyncCommitteeIndex = NextSyncCommitteeIndexElectra + 1<<NextSyncCommitteeDepthElectra
		finalizedHeaderIndex = FinalizedHeaderIndexElectra + 1<<FinalizedHeaderDepthElectra
	}

	attestedHeader, err := scu.AttestedHeader.ToConsensus()
	if err != nil {
		return false, err
	}

	finalizedHeader, err := scu.FinalizedHeader.ToConsensus()
	if err != nil {
		return false, err
	}

	finalizedHeaderRoot, err := finalizedHeader.HashTreeRoot()
	if err != nil {
		return false, err
	}

	currentSyncCommittee, err := scu.CurrentSyncCommittee.ToConsensus()
	if err != nil {
		return false, err
	}

	nextSyncCommittee, err := scu.NextSyncCommittee.ToConsensus()
	if err != nil {
		return false, err
	}
	nextSyncCommitteeRoot, err := nextSyncCommittee.HashTreeRoot()
	if err != nil {
		return false, err
	}

	nextSyncCommitteeBranch := make([][]byte, len(scu.NextSyncCommitteeBranch))
	for i, v := range scu.NextSyncCommitteeBranch {
		nextSyncCommitteeBranch[i] = make([]byte, 32)
		nextSyncCommitteeBranch[i], err = decodeHex(v)
		if err != nil {
			return false, err
		}
	}

	valid := trie.VerifyMerkleProof(attestedHeader.GetStateRoot(), nextSyncCommitteeRoot[:], uint64(nextSyncCommitteeIndex), nextSyncCommitteeBranch)
	if !valid {
		return false, nil
	}

	finalityBranch := make([][]byte, len(scu.FinalityBranch))
	for i, v := range scu.FinalityBranch {
		finalityBranch[i] = make([]byte, 32)
		finalityBranch[i], err = decodeHex(v)
		if err != nil {
			return false, err
		}
	}
	valid = trie.VerifyMerkleProof(attestedHeader.GetStateRoot(), finalizedHeaderRoot[:], uint64(finalizedHeaderIndex), finalityBranch)
	if !valid {
		return false, nil
	}

	var pubkeys []bls.PublicKey
	aggregateBytes, err := decodeHex(scu.SyncAggregate.SyncCommitteeBits)
	if err != nil {
		return false, err
	}

	aggregateBits := bitfield.Bitvector512(aggregateBytes)
	for i := uint64(0); i < aggregateBits.Len(); i++ {
		if aggregateBits.BitAt(i) {
			pubKey, err := bls.PublicKeyFromBytes(currentSyncCommittee.Pubkeys[i])
			if err != nil {
				return false, err
			}
			pubkeys = append(pubkeys, pubKey)
		}
	}

	sigBytes, err := decodeHex(scu.SyncAggregate.SyncCommitteeSignature)
	if err != nil {
		return false, err
	}
	sig, err := bls.SignatureFromBytes(sigBytes)
	if err != nil {
		return false, err
	}

	signingRoot, err := signing.ComputeSigningRoot(attestedHeader, domain[:])
	if err != nil {
		return false, err
	}
	return sig.FastAggregateVerify(pubkeys, signingRoot), nil
}

func decodeHex(hexString string) ([]byte, error) {
	if hexString[0:2] == "0x" {
		hexString = hexString[2:]
	}
	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
