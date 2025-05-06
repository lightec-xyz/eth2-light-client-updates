package src

import (
	"fmt"
	"github.com/prysmaticlabs/prysm/v5/api/server/structs"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	"github.com/prysmaticlabs/prysm/v5/config/params"
)

func BuildMainnetDomain(version string, genesis *structs.Genesis) ([]byte, error) {
	cfg := params.MainnetConfig()

	genesisValidatorsRoot, err := decodeHex(genesis.GenesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	forkVersion := cfg.GenesisForkVersion
	switch version {
	case Altair:
		forkVersion = cfg.AltairForkVersion
	case Bellatrix:
		forkVersion = cfg.BellatrixForkVersion
	case Capella:
		forkVersion = cfg.CapellaForkVersion
	case Deneb:
		forkVersion = cfg.DenebForkVersion
	case Electra:
		forkVersion = cfg.ElectraForkVersion
	default:
		return nil, fmt.Errorf("invalid version: %s", version)
	}

	domain, err := signing.ComputeDomain(cfg.DomainSyncCommittee, forkVersion, genesisValidatorsRoot[:])
	return domain, err
}

func GetMainnetDomain(version string) ([]byte, error) {
	domain := AltairDomainHex
	switch version {
	case Altair:
		domain = AltairDomainHex
	case Bellatrix:
		domain = BellatrixDomainHex
	case Capella:
		domain = CapellaDomainHex
	case Deneb:
		domain = DenebDomainHex
	case Electra:
		domain = ElectraDomainHex
	default:
		return nil, fmt.Errorf("invalid version: %s", version)
	}
	return decodeHex(domain)
}
