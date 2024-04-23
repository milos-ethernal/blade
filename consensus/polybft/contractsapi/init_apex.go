package contractsapi

import (
	"github.com/0xPolygon/polygon-edge/contracts"
)

var (
	// Apex smart contracts
	Bridge        *contracts.Artifact
	ClaimsHelper  *contracts.Artifact
	Claims        *contracts.Artifact
	SignedBatches *contracts.Artifact
	Slots         *contracts.Artifact
	UTXOsc        *contracts.Artifact
	Validators    *contracts.Artifact
)

func initApexContracts() (
	Bridge *contracts.Artifact,
	ClaimsHelper *contracts.Artifact,
	Claims *contracts.Artifact,
	SignedBatches *contracts.Artifact,
	Slots *contracts.Artifact,
	UTXOsc *contracts.Artifact,
	Validators *contracts.Artifact,
	err error) {
	Bridge, err = contracts.DecodeArtifact([]byte(BridgeArtifact))
	if err != nil {
		return
	}

	ClaimsHelper, err = contracts.DecodeArtifact([]byte(ClaimsHelperArtifact))
	if err != nil {
		return
	}

	Claims, err = contracts.DecodeArtifact([]byte(ClaimsArtifact))
	if err != nil {
		return
	}

	SignedBatches, err = contracts.DecodeArtifact([]byte(SignedBatchesArtifact))
	if err != nil {
		return
	}

	Slots, err = contracts.DecodeArtifact([]byte(SlotsArtifact))
	if err != nil {
		return
	}

	UTXOsc, err = contracts.DecodeArtifact([]byte(UTXOscArtifact))
	if err != nil {
		return
	}

	Validators, err = contracts.DecodeArtifact([]byte(ValidatorsArtifact))
	if err != nil {
		return
	}

	return
}
