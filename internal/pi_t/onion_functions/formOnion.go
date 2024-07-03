package onion_functions

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-experiment/internal/pi_t/keys"
	"golang.org/x/exp/slog"
	"hash/fnv"
	"strings"
)

const fixedLegnthOfMessage = 256

// OnionLayer represents each layer of the onion with encrypted content, header, and sepals.
type OnionLayer struct {
	Header  Header
	Content string
	Sepal   Sepal
}

type Sepal struct {
	Blocks []string
}
type Header struct {
	E string
	B []string
	A []string
}

type B struct {
	Address    string
	CypherText string
}

type CypherText struct {
	Tag       string
	Recipient string
	Layer     int
	Key       string
}

func generateSaltSpace() []byte {
	space := make([]byte, 16)
	_, err := rand.Read(space)
	if err != nil {
		panic(err)
	}
	return space
}

// FormOnion creates a forward onion from a message m, a path P, public keys pk, and metadata y.
// Parameters:
// - m: a fixed length message
// - P: a routing path (sequence of addresses representing l1 mixers and l2 gatekeepers such that len(P) = l1 + l2 + 1)
// - l1: the number of mixers in the routing path
// - l2: the number of gatekeepers in the routing path
// - pk: a list of public keys for the entities in the routing path
// - y: metadata associated with each entity (except the last destination entity) in the routing path
// Returns:
// - A list of lists of onions, O = (O_1, ..., O_l), where each O_i contains all possible variations of the i-th onion layer.
//   - The first list O_1 contains just the onion for the first mixer.
//   - For 2 <= i <= l1, the list O_i contains i options, O_i = (O_i,0, ..., O_i,i-1), each O_i,j representing the i-th onion layer with j prior bruises.
//   - For l1 + 1 <= i <= l1 + l2, the list O_i contains l1 + 1 options, depending on the total bruising from the mixers.
//   - The last list O_(l1 + l2 + 1) contains just the innermost onion for the recipient.
func FORMONION(publicKey, privateKey, m string, mixers []string, gatekeepers []string, recipient string, publicKeys []string, metadata []string, d int) ([]OnionLayer, error) {

	message := padMessage(m)

	path := append(append(append([]string{""}, mixers...), gatekeepers...), recipient)
	l1 := len(mixers)
	l2 := len(gatekeepers)
	l := l1 + l2 + 1

	// Generate keys for each layer and the master key
	layerKeys := make([][]byte, l+1)
	for i := range layerKeys {
		layerKey, _ := keys.GenerateSymmetricKey()
		layerKeys[i+1] = layerKey //base64.StdEncoding.EncodeToString(layerKey)
	}
	K, _ := keys.GenerateSymmetricKey()
	masterKey := base64.StdEncoding.EncodeToString(K)

	// Initialize the onion structure
	onionLayers := make([]OnionLayer, l+1)

	// Construct first sepal for M1
	sepal, err := createSepal(masterKey, d, layerKeys, l, l1)
	if err != nil {
		return nil, pl.WrapError(err, "failed to create sepal")
	}

	// form header and content for penultimate onion layer
	// Construct the innermost onion layer

	C_l, err := keys.EncryptWithAES(layerKeys[l], message)
	C_l_bytes, err := base64.StdEncoding.DecodeString(C_l)
	if err != nil {
		return nil, pl.WrapError(err, "failed to decode C_l")
	}

	C_arr := make([]string, l+1)
	C_arr[l], err = keys.EncryptWithAES(K, C_l_bytes)
	if err != nil {
		return nil, pl.WrapError(err, "failed to encrypt C_l_minus_1")
	}
	for i := l - 1; i >= 1; i-- {
		c_i_plus_1_bytes, err := base64.StdEncoding.DecodeString(C_arr[i+1])
		C_arr[i], err = keys.EncryptWithAES(layerKeys[i], c_i_plus_1_bytes)
		if err != nil {
			return nil, pl.WrapError(err, "failed to encrypt C_i")
		}
	}

	t_arr := make([]string, l+1)
	t_arr[l] = hash(C_l)

	E_arr := make([]string, l+1)
	E_arr[l], err = Enc(publicKeys[l-1], t_arr[l], recipient, l, layerKeys[l])
	if err != nil {
		return nil, pl.WrapError(err, "failed to encrypt ciphertext")
	}

	H_arr := make([]Header, l+1)
	H_arr[l] = Header{
		E: E_arr[l],
		B: []string{},
		A: []string{},
	}

	B_arr := make([][]string, l+1)
	for i, _ := range B_arr {
		B_arr[i] = make([]string, l+1)
	}
	B_arr[l-1][1], err = EncryptB(recipient, E_arr[l], K)
	if err != nil {
		return nil, pl.WrapError(err, "failed to encrypt B_l_minus_1_1")
	}

	onionLayers[l] = OnionLayer{
		Header:  H_arr[l],
		Content: C_arr[l],
		Sepal:   sepal,
	}

	for i := l - 1; i >= 1; i-- {
		B_arr[i][1], err = EncryptB(path[i+1], E_arr[i+1], layerKeys[i])
		if err != nil {
			return nil, pl.WrapError(err, "failed to encrypt B_i_1")
		}
		for j := 2; j <= l-j+1; j++ {
			B_arr[i][j], err = EncryptB("", B_arr[i+1][j-1], layerKeys[i])
		}
		B_i_1_to_C_i := append(B_arr[i][1:], C_arr[i])
		concat := strings.Join(B_i_1_to_C_i, "")
		t_arr[i] = hash(concat)
		role := "mixer"
		if i == l-1 {
			role = "lastGatekeeper"
		} else if i > l1 {
			role = "gatekeeper"
		}
		E_arr[i], err = Enc(publicKeys[i-1], t_arr[i], role, i, layerKeys[i]) // TODO add y_i, A_i
		H_arr[i] = Header{
			E: E_arr[i],
			B: B_arr[i],
			A: []string{},
		}

		onionLayers[i] = OnionLayer{
			Header:  H_arr[i],
			Content: C_arr[i],
			Sepal: Sepal{
				Blocks: []string{},
			},
		}
	}

	return onionLayers, nil
}

func createSepal(masterKey string, d int, layerKeys [][]byte, l int, l1 int) (Sepal, error) {
	keyBlocks, err := constructKeyBlocks(masterKey, d, layerKeys[:l])
	if err != nil {
		return Sepal{}, pl.WrapError(err, "failed to construct key blocks")
	}
	nullBlocks, err := constructKeyBlocks("", l1, layerKeys[:l1])
	if err != nil {
		return Sepal{}, pl.WrapError(err, "failed to construct null blocks")
	}
	sepalBlocks := append(keyBlocks, nullBlocks...)
	sepal := Sepal{Blocks: sepalBlocks}
	return sepal, nil
}

func EncryptB(address string, E string, layerKey []byte) (string, error) {
	b, err := json.Marshal(B{
		Address:    address,
		CypherText: E,
	})
	if err != nil {
		return "", pl.WrapError(err, "failed to marshal b")
	}
	bEncrypted, err := keys.EncryptWithAES(layerKey, b)
	return bEncrypted, nil
}

func Enc(publicKey string, tag string, role string, layer int, layerKey []byte) (string, error) {
	ciphertext := CypherText{
		Tag:       tag,
		Recipient: role,
		Layer:     layer,
		Key:       base64.StdEncoding.EncodeToString(layerKey),
	}
	cypherBytes, err := json.Marshal(ciphertext)
	if err != nil {
		return "", pl.WrapError(err, "failed to marshal ciphertext")
	}

	k_l, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", pl.WrapError(err, "failed to decode public key")
	}

	E_l, err := keys.EncryptWithAES(k_l, cypherBytes)
	if err != nil {
		return "", pl.WrapError(err, "failed to encrypt ciphertext")
	}
	return E_l, nil
}

func constructKeyBlocks(wrappedValue string, numBlocks int, layerKeys [][]byte) ([]string, error) {
	keyBlocks := make([]string, numBlocks)
	for j := 0; j < numBlocks; j++ {
		S_1, err := base64.StdEncoding.DecodeString(wrappedValue)
		if err != nil {
			return nil, pl.WrapError(err, "failed to decode inner block")
		}
		S_1 = append(S_1, generateSaltSpace()...)
		for i := len(layerKeys) - 1; i >= 0; i-- {
			k := layerKeys[i]
			innerBlockCipher, err := keys.EncryptWithAES(k, S_1)
			if err != nil {
				return nil, pl.WrapError(err, "failed to encrypt inner block")
			}
			S_1, err = base64.StdEncoding.DecodeString(innerBlockCipher)
			if err != nil {
				return nil, pl.WrapError(err, "failed to decode inner block")
			}
			S_1 = append(S_1, generateSaltSpace()...)
		}
		keyBlocks[j] = base64.StdEncoding.EncodeToString(S_1)
	}
	return keyBlocks, nil
}

func hash(s string) string {
	h := fnv.New32a()
	_, err := h.Write([]byte(s))
	if err != nil {
		slog.Error("failed to hash string", err)
		return ""
	}
	return fmt.Sprint(h.Sum32())
}

//// createSepal creates the sepal blocks for the onion layers.
//func createSepal(d, l1 int, masterKey []byte, pubKeys [][]byte) ([][]byte, error) {
//	sepals := make([][]byte, l1+1)
//	salt := make([]byte, 16)
//	for i := 0; i < l1+1; i++ {
//		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
//			panic(err)
//		}
//		if i < d {
//			// Sepal block contains the master key
//			sepals[i] = masterKey
//		} else {
//			// Sepal block contains a dummy value
//			sepals[i] = make([]byte, 32)
//		}
//		for j := 0; j < len(pubKeys); j++ {
//			cipherText, err := keys.EncryptWithAES(pubKeys[j], append(sepals[i], salt...))
//			if err != nil {
//				return nil, pl.WrapError(err, "failed to encrypt sepal block")
//			}
//			sepals[i], err = base64.StdEncoding.DecodeString(cipherText)
//			if err != nil {
//				return nil, pl.WrapError(err, "failed to decode sepal block")
//			}
//		}
//	}
//	return sepals, nil
//}
//
//// FormOnion creates a forward onion from a message m, a path P, public keys pk, and metadata y.
//// Parameters:
//// - m: a fixed length message
//// - P: a routing path (sequence of addresses representing l1 mixers and l2 gatekeepers such that len(P) = l1 + l2 + 1)
//// - l1: the number of mixers in the routing path
//// - l2: the number of gatekeepers in the routing path
//// - pk: a list of public keys for the entities in the routing path
//// - y: metadata associated with each entity (except the last destination entity) in the routing path
//// Returns:
//// - A list of lists of onions, O = (O_1, ..., O_l), where each O_i contains all possible variations of the i-th onion layer.
////   - The first list O_1 contains just the onion for the first mixer.
////   - For 2 <= i <= l1, the list O_i contains i options, O_i = (O_i,0, ..., O_i,i-1), each O_i,j representing the i-th onion layer with j prior bruises.
////   - For l1 + 1 <= i <= l1 + l2, the list O_i contains l1 + 1 options, depending on the total bruising from the mixers.
////   - The last list O_(l1 + l2 + 1) contains just the innermost onion for the recipient.
//func FormOnion(m string, P []string, l1, l2 int, pk []string, y []string) (O [][]OnionLayer, err error) {
//	paddedMessage := padMessage(m)
//
//	// Convert public keys and metadata to byte slices
//	publicKeys := make([][]byte, len(pk))
//	metadata := make([][]byte, len(y))
//	for i := range publicKeys {
//		publicKeys[i] = []byte(pk[i])
//	}
//	for i := range metadata {
//		metadata[i] = []byte(y[i])
//	}
//
//	// Generate keys for each layer and the master key
//	layerKeys := make([][]byte, l1+l2+1)
//	for i := range layerKeys {
//		layerKeys[i], _ = keys.GenerateSymmetricKey()
//	}
//	masterKey, _ := keys.GenerateSymmetricKey()
//
//	// Initialize the onion structure
//	onionLayers := make([][]OnionLayer, l1+l2+1)
//
//	// Create the first sepal
//	sepals, err := createSepal(l1, l1, masterKey, layerKeys)
//	if err != nil {
//		return nil, pl.WrapError(err, "failed to create sepal")
//	}
//
//	// Create the first onion layer with all variations for the first mixer
//	onionLayers[0] = make([]OnionLayer, 1)
//	header, _ := keys.EncryptWithAES(publicKeys[0], []byte(fmt.Sprintf("Mixer %d", 1)))
//	content, _ := keys.EncryptWithAES(layerKeys[0], paddedMessage)
//	onionLayers[0][0] = OnionLayer{
//		Content: content,
//		Header:  header,
//		Sepal:   sepals,
//	}
//
//	// Create layers for remaining mixers
//	for i := 1; i <= l1; i++ {
//		variations := i + 1
//		onionLayers[i] = make([]OnionLayer, variations)
//		for j := 0; j < variations; j++ {
//			sepalCopy := make([][]byte, len(sepals)-1)
//			copy(sepalCopy, sepals[1:])
//			header, _ := keys.EncryptWithAES(publicKeys[i], []byte(fmt.Sprintf("Mixer %d", i+1)))
//			content, _ := keys.EncryptWithAES(layerKeys[i], []byte(onionLayers[i-1][j].Content))
//			onionLayers[i][j] = OnionLayer{
//				Content: content,
//				Header:  header,
//				Sepal:   sepalCopy,
//			}
//		}
//	}
//
//	// Create layers for gatekeepers
//	for i := l1 + 1; i <= l1+l2; i++ {
//		variations := l1 + 1
//		onionLayers[i] = make([]OnionLayer, variations)
//		for j := 0; j < variations; j++ {
//			header, _ := keys.EncryptWithAES(publicKeys[i], []byte(fmt.Sprintf("Gatekeeper %d", i-l1)))
//			content, _ := keys.EncryptWithAES(layerKeys[i], []byte(onionLayers[i-1][j].Content))
//			onionLayers[i][j] = OnionLayer{
//				Content: content,
//				Header:  header,
//				Sepal:   sepals[:len(sepals)-1],
//			}
//		}
//	}
//
//	// The last layer for the recipient
//	onionLayers[l1+l2] = make([]OnionLayer, 1)
//	header, _ = keys.EncryptWithAES(publicKeys[len(publicKeys)-1], []byte("Recipient"))
//	content, _ = keys.EncryptWithAES(layerKeys[l1+l2], paddedMessage)
//	onionLayers[l1+l2][0] = OnionLayer{
//		Content: content,
//		Header:  header,
//		Sepal:   sepals[:1],
//	}
//
//	return onionLayers
//
//}

func padMessage(message string) []byte {
	var nullTerminator byte = '\000'
	var paddedMessage = make([]byte, fixedLegnthOfMessage)
	var mLength = len(message)

	for i := 0; i < fixedLegnthOfMessage; i++ {
		if i >= mLength || i == fixedLegnthOfMessage-1 {
			paddedMessage[i] = nullTerminator
		} else {
			paddedMessage[i] = message[i]
		}
	}
	return paddedMessage
}
