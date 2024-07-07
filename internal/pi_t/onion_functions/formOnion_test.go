package onion_functions

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-experiment/internal/api/structs"
	"github.com/HannahMarsh/pi_t-experiment/internal/pi_t/keys"
	"github.com/HannahMarsh/pi_t-experiment/pkg/utils"
	"golang.org/x/exp/slog"
	"strings"
	"testing"
)

func TestFormSepal(t *testing.T) {
	pl.SetUpLogrusAndSlog("debug")
	l1 := 5
	l2 := 5
	d := 3
	l := l1 + l2 + 1

	// Generate keys for each layer and the master key
	layerKeys := make([][]byte, l+1)
	for i := range layerKeys {
		layerKey, _ := keys.GenerateSymmetricKey()
		layerKeys[i] = layerKey //base64.StdEncoding.EncodeToString(layerKey)
	}
	K, err := keys.GenerateSymmetricKey()
	if err != nil {
		slog.Error("failed to generate symmetric key", err)
		t.Fatalf("GenerateSymmetricKey() error: %v", err)
	}
	masterKey := base64.StdEncoding.EncodeToString(K)

	// Construct first sepal for M1
	sepal, _, err := formSepal(masterKey, d, layerKeys, l, l1)
	if err != nil {
		slog.Error("failed to create sepal", err)
		t.Fatalf("formSepal() error: %v", err)
	}

	if len(sepal.Blocks) != l1+1 {
		t.Fatalf("formSepal() expected %d blocks, got %d", l1+1, len(sepal.Blocks))
	}

	for j, sepalBlock := range sepal.Blocks {

		if j < d {
			decryptedBlock, _, err := keys.DecryptStringWithAES(layerKeys[1], sepalBlock)
			if err != nil {
				slog.Error("failed to decrypt sepal block", err)
				t.Fatalf("DecryptStringWithAES() error: %v", err)
			}
			for i := 2; i <= l-1; i++ {
				k := layerKeys[i]

				decryptedBlock, _, err = keys.DecryptWithAES(k, decryptedBlock)
				if err != nil {
					slog.Error("failed to decrypt sepal block", err)
					t.Fatalf("DecryptStringWithAES() error: %v", err)
				}
			}
			for index, b := range K {
				keyblockByte := decryptedBlock[index]
				if keyblockByte != b {
					slog.Info("failed to decrypt sepal block. Expected master key")
					t.Fatalf("formSepal() expected keyblock %v, got %v", b, keyblockByte)
				}
			}
		} else {
			decryptedBlock, _, err := keys.DecryptStringWithAES(layerKeys[1], sepalBlock)
			if err != nil {
				slog.Error("failed to decrypt sepal block", err)
				t.Fatalf("DecryptStringWithAES() error: %v", err)
			}
			var decryptedString string
			for i := 2; i <= l1+1; i++ {
				k := layerKeys[i]

				decryptedBlock, decryptedString, err = keys.DecryptWithAES(k, decryptedBlock)
				if err != nil {
					slog.Error("failed to decrypt sepal block", err)
					t.Fatalf("DecryptStringWithAES() error: %v", err)
				}
			}
			if !strings.HasPrefix(decryptedString, "null") {
				t.Fatalf("formSepal() expected decryptedString to start with 'null', got %v", decryptedString)
			}
			//if len(decryptedString) != (saltLength * 8) {
			//	t.Fatalf("formSepal() expected decryptedString length %v, got %v", (saltLength * 8), len(decryptedString))
			//}
		}
	}
}

func TestBruiseSepal(t *testing.T) {
	pl.SetUpLogrusAndSlog("debug")
	l1 := 5
	l2 := 5
	d := 3
	l := l1 + l2 + 1

	// Generate keys for each layer and the master key
	layerKeys := make([][]byte, l+1)
	for i := range layerKeys {
		layerKey, _ := keys.GenerateSymmetricKey()
		layerKeys[i] = layerKey //base64.StdEncoding.EncodeToString(layerKey)
	}
	K, err := keys.GenerateSymmetricKey()
	if err != nil {
		slog.Error("failed to generate symmetric key", err)
		t.Fatalf("GenerateSymmetricKey() error: %v", err)
	}
	masterKey := base64.StdEncoding.EncodeToString(K)

	// Construct first sepal for M1
	sepal, _, err := formSepal(masterKey, d, layerKeys, l, l1)
	if err != nil {
		slog.Error("failed to create sepal", err)
		t.Fatalf("formSepal() error: %v", err)
	}

	if len(sepal.Blocks) != l1+1 {
		t.Fatalf("formSepal() expected %d blocks, got %d", l1+1, len(sepal.Blocks))
	}

	bruised, err := bruiseSepal(sepal, layerKeys, d-1, l1, l, d)
	if err != nil {
		slog.Error("failed to bruise sepal", err)
		t.Fatalf("bruiseSepal() error: %v", err)
	}

	if len(bruised.Blocks) != 1 {
		t.Fatalf("bruiseSepal() expected 1 block, got %d", len(bruised.Blocks))
	}

	block := bruised.Blocks[0]

	for i := l1 + 1; i <= l-1; i++ {
		_, block, err = keys.DecryptStringWithAES(layerKeys[i], block)
		if err != nil {
			slog.Error("failed to decrypt sepal block", err)
			t.Fatalf("DecryptStringWithAES() error: %v", err)
		}
		slog.Info("decrypted block: ", "block", block)
	}

	slog.Info("bruised sepal block: ", "block", block)

	keyblockBytes, err := base64.StdEncoding.DecodeString(block)
	if err != nil {
		slog.Error("failed to decode sepal block", err)
		t.Fatalf("DecodeString() error: %v", err)
	}

	for index, b := range K {
		keyblockByte := keyblockBytes[index]
		if keyblockByte != b {
			slog.Info("failed to decrypt sepal block. Expected master key")
			t.Fatalf("formSepal() expected keyblock %v, got %v", b, keyblockByte)
		}
	}

	// bruise it d times
	bruised, err = bruiseSepal(sepal, layerKeys, d, l1, l, d)
	if err != nil {
		slog.Error("failed to bruise sepal", err)
		t.Fatalf("bruiseSepal() error: %v", err)
	}

	if len(bruised.Blocks) != 1 {
		t.Fatalf("bruiseSepal() expected 1 block, got %d", len(bruised.Blocks))
	}

	block = bruised.Blocks[0]

	_, block, err = keys.DecryptStringWithAES(layerKeys[l1+1], block)
	if err != nil {
		slog.Error("failed to decrypt sepal block", err)
		t.Fatalf("DecryptStringWithAES() error: %v", err)
	}
	slog.Info("decrypted block: ", "block", block)

	if !strings.HasPrefix(block, "null") {
		t.Fatalf("formSepal() expected decryptedString to start with 'null', got %v", bruised.Blocks[0])
	}
}

func bruiseSepal(sepal Sepal, layerKeys [][]byte, numBruises int, l1 int, l int, d int) (s Sepal, err error) {
	randomBools := make([]bool, l1)
	for i := range randomBools {
		if i < numBruises {
			randomBools[i] = true
		} else {
			randomBools[i] = false
		}
	}
	utils.Shuffle(randomBools)
	randomBools = append([]bool{false}, randomBools...)

	for i := 1; i <= l1; i++ {
		dobruiseSepal := false
		if i <= l1 {
			dobruiseSepal = randomBools[i]
		}
		sepal, err = sepal.PeelSepal(layerKeys[i], dobruiseSepal, d)
		if err != nil {
			return Sepal{}, pl.WrapError(err, "failed to peel sepal")
		}
	}
	return sepal, nil
}

func TestSepalHashes(t *testing.T) {
	pl.SetUpLogrusAndSlog("debug")
	l1 := 5
	l2 := 5
	d := 3
	l := l1 + l2 + 1

	// Generate keys for each layer and the master key
	layerKeys := make([][]byte, l+1)
	for i := range layerKeys {
		layerKey, _ := keys.GenerateSymmetricKey()
		layerKeys[i] = layerKey //base64.StdEncoding.EncodeToString(layerKey)
	}
	K, err := keys.GenerateSymmetricKey()
	if err != nil {
		slog.Error("failed to generate symmetric key", err)
		t.Fatalf("GenerateSymmetricKey() error: %v", err)
	}
	masterKey := base64.StdEncoding.EncodeToString(K)

	// Construct first sepal for M1
	sepal, A, err := formSepal(masterKey, d, layerKeys, l, l1)
	if err != nil {
		slog.Error("failed to create sepal", err)
		t.Fatalf("formSepal() error: %v", err)
	}

	h := hash(strings.Join(sepal.Blocks, ""))
	if A[0][0] != h {
		t.Fatalf("hash not expected")
	}

	for i := 1; i <= l1; i++ {
		sepal, err = sepal.PeelSepal(layerKeys[i], false, d)
		if err != nil {
			slog.Error("failed to peel sepal", err)
			t.Fatalf("PeelSepal() error: %v", err)
		}
		h = hash(strings.Join(sepal.Blocks, ""))
		if !utils.Contains(A[i], func(str string) bool {
			return str == h
		}) {
			t.Fatalf("hash not expected")
		}
	}

}

func TestFORMONION(t *testing.T) {
	pl.SetUpLogrusAndSlog("debug")

	var err error

	l1 := 5
	l2 := 5
	d := 3
	l := l1 + l2 + 1

	type node struct {
		privateKeyPEM string
		publicKeyPEM  string
		address       string
	}

	nodes := make([]node, l+1)

	for i := 0; i < l+1; i++ {
		privateKeyPEM, publicKeyPEM, err := keys.KeyGen()
		if err != nil {
			t.Fatalf("KeyGen() error: %v", err)
		}
		nodes[i] = node{privateKeyPEM, publicKeyPEM, fmt.Sprintf("node%d", i)}
	}

	secretMessage := "secret message"

	payload, err := json.Marshal(structs.Message{
		Msg:  secretMessage,
		To:   nodes[l].address,
		From: nodes[0].address,
	})
	if err != nil {
		slog.Error("json.Marshal() error", err)
		t.Fatalf("json.Marshal() error: %v", err)
	}

	publicKeys := utils.Map(nodes[1:], func(n node) string { return n.publicKeyPEM })
	routingPath := utils.Map(nodes[1:], func(n node) string { return n.address })

	onion, err := FORMONION(nodes[0].publicKeyPEM, nodes[0].privateKeyPEM, string(payload), routingPath[1:l1], routingPath[l1:len(routingPath)-1], routingPath[len(routingPath)-1], publicKeys[1:], []string{}, d)
	if err != nil {
		slog.Error("", err)
		t.Fatalf("failed")
	}
	slog.Info("", "", onion)

}
