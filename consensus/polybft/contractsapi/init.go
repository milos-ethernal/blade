package contractsapi

import (
	"embed"
	"fmt"
	"log"
	"path"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi/artifact"
)

const (
	testContractsDir = "test-contracts"
)

var (
	// Blade smart contracts
	CheckpointManager               *artifact.Artifact
	ExitHelper                      *artifact.Artifact
	StateSender                     *artifact.Artifact
	RootERC20Predicate              *artifact.Artifact
	RootERC721Predicate             *artifact.Artifact
	RootERC1155Predicate            *artifact.Artifact
	ChildMintableERC20Predicate     *artifact.Artifact
	ChildMintableERC721Predicate    *artifact.Artifact
	ChildMintableERC1155Predicate   *artifact.Artifact
	BLS                             *artifact.Artifact
	BLS256                          *artifact.Artifact
	System                          *artifact.Artifact
	Merkle                          *artifact.Artifact
	ChildValidatorSet               *artifact.Artifact
	NativeERC20                     *artifact.Artifact
	NativeERC20Mintable             *artifact.Artifact
	StateReceiver                   *artifact.Artifact
	ChildERC20                      *artifact.Artifact
	ChildERC20Predicate             *artifact.Artifact
	ChildERC20PredicateACL          *artifact.Artifact
	RootMintableERC20Predicate      *artifact.Artifact
	RootMintableERC20PredicateACL   *artifact.Artifact
	ChildERC721                     *artifact.Artifact
	ChildERC721Predicate            *artifact.Artifact
	ChildERC721PredicateACL         *artifact.Artifact
	RootMintableERC721Predicate     *artifact.Artifact
	RootMintableERC721PredicateACL  *artifact.Artifact
	ChildERC1155                    *artifact.Artifact
	ChildERC1155Predicate           *artifact.Artifact
	ChildERC1155PredicateACL        *artifact.Artifact
	RootMintableERC1155Predicate    *artifact.Artifact
	RootMintableERC1155PredicateACL *artifact.Artifact
	L2StateSender                   *artifact.Artifact
	CustomSupernetManager           *artifact.Artifact
	StakeManager                    *artifact.Artifact
	EpochManager                    *artifact.Artifact
	RootERC721                      *artifact.Artifact
	RootERC1155                     *artifact.Artifact
	EIP1559Burn                     *artifact.Artifact
	BladeManager                    *artifact.Artifact
	GenesisProxy                    *artifact.Artifact
	TransparentUpgradeableProxy     *artifact.Artifact

	// Governance
	NetworkParams *artifact.Artifact
	ForkParams    *artifact.Artifact
	ChildGovernor *artifact.Artifact
	ChildTimelock *artifact.Artifact

	// test smart contracts
	//go:embed test-contracts/*
	testContracts          embed.FS
	TestWriteBlockMetadata *artifact.Artifact
	RootERC20              *artifact.Artifact
	TestSimple             *artifact.Artifact
	TestRewardToken        *artifact.Artifact

	contractArtifacts = map[string]*artifact.Artifact{
		"CheckpointManager":               CheckpointManager,
		"ExitHelper":                      ExitHelper,
		"StateSender":                     StateSender,
		"RootERC20Predicate":              RootERC20Predicate,
		"RootERC721Predicate":             RootERC721Predicate,
		"RootERC1155Predicate":            RootERC1155Predicate,
		"ChildMintableERC20Predicate":     ChildMintableERC20Predicate,
		"ChildMintableERC721Predicate":    ChildMintableERC721Predicate,
		"ChildMintableERC1155Predicate":   ChildMintableERC1155Predicate,
		"BLS":                             BLS,
		"BLS256":                          BLS256,
		"System":                          System,
		"Merkle":                          Merkle,
		"ChildValidatorSet":               ChildValidatorSet,
		"NativeERC20":                     NativeERC20,
		"NativeERC20Mintable":             NativeERC20Mintable,
		"StateReceiver":                   StateReceiver,
		"ChildERC20":                      ChildERC20,
		"ChildERC20Predicate":             ChildERC20Predicate,
		"ChildERC20PredicateACL":          ChildERC20PredicateACL,
		"RootMintableERC20Predicate":      RootMintableERC20Predicate,
		"RootMintableERC20PredicateACL":   RootMintableERC20PredicateACL,
		"ChildERC721":                     ChildERC721,
		"ChildERC721Predicate":            ChildERC721Predicate,
		"ChildERC721PredicateACL":         ChildERC721PredicateACL,
		"RootMintableERC721Predicate":     RootMintableERC721Predicate,
		"RootMintableERC721PredicateACL":  RootMintableERC721PredicateACL,
		"ChildERC1155":                    ChildERC1155,
		"ChildERC1155Predicate":           ChildERC1155Predicate,
		"ChildERC1155PredicateACL":        ChildERC1155PredicateACL,
		"RootMintableERC1155Predicate":    RootMintableERC1155Predicate,
		"RootMintableERC1155PredicateACL": RootMintableERC1155PredicateACL,
		"L2StateSender":                   L2StateSender,
		"CustomSupernetManager":           CustomSupernetManager,
		"StakeManager":                    StakeManager,
		"EpochManager":                    EpochManager,
		"RootERC721":                      RootERC721,
		"RootERC1155":                     RootERC1155,
		"EIP1559Burn":                     EIP1559Burn,
		"BladeManager":                    BladeManager,
		"GenesisProxy":                    GenesisProxy,
		"TransparentUpgradeableProxy":     TransparentUpgradeableProxy,
		"NetworkParams":                   NetworkParams,
		"ForkParams":                      ForkParams,
		"ChildGovernor":                   ChildGovernor,
		"ChildTimelock":                   ChildTimelock,
		"TestWriteBlockMetadata":          TestWriteBlockMetadata,
		"RootERC20":                       RootERC20,
		"TestSimple":                      TestSimple,
		"TestRewardToken":                 TestRewardToken,
	}
)

func init() {
	var err error

	CheckpointManager, err = artifact.DecodeArtifact([]byte(CheckpointManagerArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ExitHelper, err = artifact.DecodeArtifact([]byte(ExitHelperArtifact))
	if err != nil {
		log.Fatal(err)
	}

	L2StateSender, err = artifact.DecodeArtifact([]byte(L2StateSenderArtifact))
	if err != nil {
		log.Fatal(err)
	}

	BLS, err = artifact.DecodeArtifact([]byte(BLSArtifact))
	if err != nil {
		log.Fatal(err)
	}

	BLS256, err = artifact.DecodeArtifact([]byte(BN256G2Artifact))
	if err != nil {
		log.Fatal(err)
	}

	Merkle, err = artifact.DecodeArtifact([]byte(MerkleArtifact))
	if err != nil {
		log.Fatal(err)
	}

	StateSender, err = artifact.DecodeArtifact([]byte(StateSenderArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC20Predicate, err = artifact.DecodeArtifact([]byte(RootERC20PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC721Predicate, err = artifact.DecodeArtifact([]byte(RootERC721PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC1155Predicate, err = artifact.DecodeArtifact([]byte(RootERC1155PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildMintableERC20Predicate, err = artifact.DecodeArtifact([]byte(ChildMintableERC20PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildMintableERC721Predicate, err = artifact.DecodeArtifact([]byte(ChildMintableERC721PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildMintableERC1155Predicate, err = artifact.DecodeArtifact([]byte(ChildMintableERC1155PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	StateReceiver, err = artifact.DecodeArtifact([]byte(StateReceiverArtifact))
	if err != nil {
		log.Fatal(err)
	}

	System, err = artifact.DecodeArtifact([]byte(SystemArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC20, err = artifact.DecodeArtifact([]byte(ChildERC20Artifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC20Predicate, err = artifact.DecodeArtifact([]byte(ChildERC20PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC20PredicateACL, err = artifact.DecodeArtifact([]byte(ChildERC20PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC20Predicate, err = artifact.DecodeArtifact([]byte(RootMintableERC20PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC20PredicateACL, err = artifact.DecodeArtifact([]byte(RootMintableERC20PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC721, err = artifact.DecodeArtifact([]byte(ChildERC721Artifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC721Predicate, err = artifact.DecodeArtifact([]byte(ChildERC721PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC721PredicateACL, err = artifact.DecodeArtifact([]byte(ChildERC721PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC721Predicate, err = artifact.DecodeArtifact([]byte(RootMintableERC721PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC721PredicateACL, err = artifact.DecodeArtifact([]byte(RootMintableERC721PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC1155, err = artifact.DecodeArtifact([]byte(ChildERC1155Artifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC1155Predicate, err = artifact.DecodeArtifact([]byte(ChildERC1155PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildERC1155PredicateACL, err = artifact.DecodeArtifact([]byte(ChildERC1155PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC1155Predicate, err = artifact.DecodeArtifact([]byte(RootMintableERC1155PredicateArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootMintableERC1155PredicateACL, err = artifact.DecodeArtifact([]byte(RootMintableERC1155PredicateACLArtifact))
	if err != nil {
		log.Fatal(err)
	}

	NativeERC20, err = artifact.DecodeArtifact([]byte(NativeERC20Artifact))
	if err != nil {
		log.Fatal(err)
	}

	NativeERC20Mintable, err = artifact.DecodeArtifact([]byte(NativeERC20MintableArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC20, err = artifact.DecodeArtifact([]byte(MockERC20Artifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC721, err = artifact.DecodeArtifact([]byte(MockERC721Artifact))
	if err != nil {
		log.Fatal(err)
	}

	RootERC1155, err = artifact.DecodeArtifact([]byte(MockERC1155Artifact))
	if err != nil {
		log.Fatal(err)
	}

	TestWriteBlockMetadata, err = artifact.DecodeArtifact(readTestContractContent("TestWriteBlockMetadata.json"))
	if err != nil {
		log.Fatal(err)
	}

	TestSimple, err = artifact.DecodeArtifact(readTestContractContent("TestSimple.json"))
	if err != nil {
		log.Fatal(err)
	}

	TestRewardToken, err = artifact.DecodeArtifact(readTestContractContent("TestRewardToken.json"))
	if err != nil {
		log.Fatal(err)
	}

	StakeManager, err = artifact.DecodeArtifact([]byte(StakeManagerArtifact))
	if err != nil {
		log.Fatal(err)
	}

	EpochManager, err = artifact.DecodeArtifact([]byte(EpochManagerArtifact))
	if err != nil {
		log.Fatal(err)
	}

	EIP1559Burn, err = artifact.DecodeArtifact([]byte(EIP1559BurnArtifact))
	if err != nil {
		log.Fatal(err)
	}

	BladeManager, err = artifact.DecodeArtifact([]byte(BladeManagerArtifact))
	if err != nil {
		log.Fatal(err)
	}

	GenesisProxy, err = artifact.DecodeArtifact([]byte(GenesisProxyArtifact))
	if err != nil {
		log.Fatal(err)
	}

	TransparentUpgradeableProxy, err = artifact.DecodeArtifact([]byte(TransparentUpgradeableProxyArtifact))
	if err != nil {
		log.Fatal(err)
	}

	NetworkParams, err = artifact.DecodeArtifact([]byte(NetworkParamsArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ForkParams, err = artifact.DecodeArtifact([]byte(ForkParamsArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildGovernor, err = artifact.DecodeArtifact([]byte(ChildGovernorArtifact))
	if err != nil {
		log.Fatal(err)
	}

	ChildTimelock, err = artifact.DecodeArtifact([]byte(ChildTimelockArtifact))
	if err != nil {
		log.Fatal(err)
	}
}

func readTestContractContent(contractFileName string) []byte {
	contractRaw, err := testContracts.ReadFile(path.Join(testContractsDir, contractFileName))
	if err != nil {
		log.Fatal(err)
	}

	return contractRaw
}

func GetArtifactFromArtifactName(artifactName string) (*artifact.Artifact, error) {
	if contractArtifact, ok := contractArtifacts[artifactName]; ok {
		return contractArtifact, nil
	}

	return nil, fmt.Errorf("")
}
