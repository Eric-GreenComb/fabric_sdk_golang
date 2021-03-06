package encrypt

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"os"

	"errors"

	"github.com/fabric_sdk_golang/core/crypto/primitives"
	"github.com/fabric_sdk_golang/ecies"
	pb "github.com/fabric_sdk_golang/protos"
	"github.com/op/go-logging"
)

var (
	logger = logging.MustGetLogger("offical encrypt")
	pk     *ecdsa.PublicKey
)

const ( // pk of chain, for encrypt transaction

	chainKey = `-----BEGIN ECDSA PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEliN7kTaC2P2GHsJZs/ZphlNKbQmH
jCXOiPEaeRMkbfbr4c04R6Cl9AYv5md1J4Xo+nbz/VX2Qtu7UDzwfDMQeQ==
-----END ECDSA PUBLIC KEY-----`
)

type chainCodeValidatorMessage1_2 struct {
	PrivateKey []byte
	StateKey   []byte
}

func init() {

	format := logging.MustStringFormatter(`[%{module}] %{time:2006-01-02 15:04:05} [%{level}] [%{longpkg} %{shortfile}] { %{message} }`)

	backendConsole := logging.NewLogBackend(os.Stderr, "", 0)
	backendConsole2Formatter := logging.NewBackendFormatter(backendConsole, format)

	logging.SetBackend(backendConsole2Formatter)

	block, _ := pem.Decode([]byte(chainKey))
	if block == nil {
		logger.Fatal("pem.Decode return nil")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.Fatal(err)
	}

	var ok bool
	if pk, ok = pub.(*ecdsa.PublicKey); !ok {
		logger.Fatal("chainKey is not in format of ecdsa")
	}
}

func Process(tx *pb.Transaction, pkChain *ecdsa.PublicKey) error {

	if pkChain == nil {
		logger.Error("控指针引用！")
		errors.New("控指针引用！")
	}
	priv, err := ecdsa.GenerateKey(primitives.GetDefaultCurve(), rand.Reader)
	if err != nil {
		logger.Error(err)
		return err
	}

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		logger.Error(err)
		return err
	}

	var stateKey []byte
	switch tx.Type {
	case pb.Transaction_CHAINCODE_DEPLOY:
		// Prepare chaincode stateKey and privateKey
		stateKey, err = primitives.GenAESKey()
		if err != nil {
			logger.Error(err)
			return err
		}
	case pb.Transaction_CHAINCODE_QUERY:
		// Prepare chaincode stateKey and privateKey
		stateKey = primitives.HMACAESTruncated(nil, append([]byte{6}, tx.Nonce...))
	case pb.Transaction_CHAINCODE_INVOKE:
		// Prepare chaincode stateKey and privateKey
		stateKey = make([]byte, 0)
	}

	msgToValidators, err := asn1.Marshal(chainCodeValidatorMessage1_2{privBytes, stateKey})
	if err != nil {
		logger.Error(err)
		return err
	}

	encMsgToValidators, err := ecies.Encrypt(rand.Reader, pkChain, msgToValidators)
	if err != nil {
		logger.Error(err)
		return err
	}
	tx.ToValidators = encMsgToValidators

	encryptedChaincodeID, err := ecies.Encrypt(rand.Reader, &priv.PublicKey, tx.ChaincodeID)
	if err != nil {
		logger.Error(err)
		return err
	}
	tx.ChaincodeID = encryptedChaincodeID

	encryptedPayload, err := ecies.Encrypt(rand.Reader, &priv.PublicKey, tx.Payload)
	if err != nil {
		logger.Error(err)
		return err
	}
	tx.Payload = encryptedPayload

	if len(tx.Metadata) != 0 {
		encryptedMetadata, err := ecies.Encrypt(rand.Reader, &priv.PublicKey, tx.Metadata)
		if err != nil {
			logger.Error(err)
			return err
		}
		tx.Metadata = encryptedMetadata
	}

	return nil
}
