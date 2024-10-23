package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/HannahMarsh/pi_t-experiment/config"
	"github.com/HannahMarsh/pi_t-experiment/internal/api/api_functions"
	"github.com/HannahMarsh/pi_t-experiment/internal/api/structs"
	"github.com/HannahMarsh/pi_t-experiment/internal/metrics"
	"github.com/HannahMarsh/pi_t-experiment/internal/pi_t/crypto/keys"
	"github.com/HannahMarsh/pi_t-experiment/internal/pi_t/onion_model"
	"github.com/HannahMarsh/pi_t-experiment/pkg/cm"
	"github.com/HannahMarsh/pi_t-experiment/pkg/utils"
	"io"
	"net/http"
	"sync"
	"time"

	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-experiment/internal/pi_t"
	"log/slog"
)

// Relay represents a participating relay in the network.
type Relay struct {
	ID                     int                         // Unique identifier for the relay.
	Host                   string                      // Host address of the relay.
	Port                   int                         // Port number on which the relay listens.
	Address                string                      // Full address of the relay in the form http://host:port.
	PrivateKey             string                      // Relay's private key for decryption.
	PublicKey              string                      // Relay's public key for encryption.
	PrometheusPort         int                         // Port number for Prometheus metrics.
	BulletinBoardUrl       string                      // URL of the bulletin board for relay registration and communication.
	lastUpdate             time.Time                   // Timestamp of the last update sent to the bulletin board.
	checkpointsReceived    *cm.ConcurrentMap[int, int] // Concurrent map to track the number of received checkpoints per layer.
	onionsToSend           map[int][]queuedOnion       // List of onions to send to the next hop.
	currentLayer           int                         // Current layer of the relay.
	expectedNonces         []map[string]bool           // List of expected nonces for each layer, used to verify received onions.
	expectedNumCheckpoints map[int]int                 // map to track the number of expected checkpoints per layer.
	isCorrupted            bool                        // Flag indicating whether the relay is corrupted (meaning it does not perform any mixing).
	wg                     sync.WaitGroup
	mu                     sync.RWMutex
	addressToDropFrom      string
	ShutdownMetrics        func()
}

type queuedOnion struct {
	onion          onion_model.Onion
	nextHop        string
	layer          int
	originallySent int64
	timeReceived   time.Time
}

// NewRelay creates a new relay instance with a unique ID, host, and port.
func NewRelay(id int, host string, port int, promPort int, bulletinBoardUrl string) (*Relay, error) {
	// Generate a key pair (private and public) for the relay.
	if privateKey, publicKey, err := keys.KeyGen(); err != nil {
		return nil, pl.WrapError(err, "relay.NewClient(): failed to generate key pair")
	} else {
		// Initialize a list of expected nonces for each layer.
		expectedCheckpoints := make([]map[string]bool, config.GetL1()+config.GetL2()+1)
		for i := range expectedCheckpoints {
			expectedCheckpoints[i] = make(map[string]bool)
		}

		n := &Relay{
			ID:                     id,
			Host:                   host,
			Address:                fmt.Sprintf("http://%s:%d", host, port),
			Port:                   port,
			PublicKey:              publicKey,
			PrivateKey:             privateKey,
			PrometheusPort:         promPort,
			BulletinBoardUrl:       bulletinBoardUrl,
			checkpointsReceived:    &cm.ConcurrentMap[int, int]{},
			expectedNumCheckpoints: make(map[int]int),
			expectedNonces:         expectedCheckpoints,
			onionsToSend:           make(map[int][]queuedOnion, 0),
			ShutdownMetrics:        func() {},
		}
		n.wg.Add(1)

		// Register the relay with the bulletin board.
		if err2 := n.RegisterWithBulletinBoard(); err2 != nil {
			return nil, pl.WrapError(err2, "relay.NewRelay(): failed to register with bulletin board")
		}

		// Start periodic updates to the bulletin board.
		go n.StartPeriodicUpdates(time.Second * 3)

		return n, nil
	}
}

// getPublicNodeInfo returns the relay's public information in the form of a PublicNodeApi struct.
func (n *Relay) getPublicNodeInfo() structs.PublicNodeApi {
	t, _ := utils.GetTimestamp() // Record the current time for the update.
	return structs.PublicNodeApi{
		ID:             n.ID,
		Address:        n.Address,
		PublicKey:      n.PublicKey,
		PrometheusPort: n.PrometheusPort,
		Host:           n.Host,
		Port:           n.Port,
		Time:           t,
	}
}

// StartPeriodicUpdates periodically updates the relay's status on the bulletin board.
func (n *Relay) StartPeriodicUpdates(interval time.Duration) {
	ticker := time.NewTicker(interval) // Create a ticker that triggers updates at the specified interval.
	go func() {
		for range ticker.C {
			// Update the bulletin board with the relay's current status.
			if err := n.updateBulletinBoard("/updateRelay", http.StatusOK); err != nil {
				fmt.Printf("Error updating bulletin board: %v\n", err)
				return
			}
		}
	}()
}

// startRun initializes a run based on the start signal received from the bulletin board.
func (n *Relay) startRun(start structs.RelayStartRunApi) (didParticipate bool, e error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	defer n.wg.Done()

	n.ShutdownMetrics()
	n.ShutdownMetrics = metrics.ServeMetrics(start.StartOfRun, n.PrometheusPort, metrics.ONION_SIZE, metrics.LATENCY_BETWEEN_HOPS, metrics.PROCESSING_TIME)

	config.UpdateConfig(start.Config) // Update the global configuration based on the start signal.

	// Determine if the relay is corrupted based on the configuration's corruption rate (Chi).
	numRelays := utils.Max(config.GetMinimumRelays(), n.ID)
	numCorrupted := int(config.GetChi() * float64(numRelays))
	corruptedNodes := utils.PseudoRandomSubset(utils.NewIntArray(1, numRelays), numCorrupted, 42)
	isCorrupted := utils.Contains(corruptedNodes, func(id int) bool {
		return id == n.ID
	})

	addressToDropFrom := ""
	//
	//// If the relay is corrupted, set the address to drop all onions from (specified in the configuration)
	//if isCorrupted {
	//	if c := utils.Find(config.flobalConfig.Clients, func(client config2.Client) bool {
	//		return client.ID == config.flobalConfig.DropAllOnionsFromClient
	//	}); c != nil {
	//		addressToDropFrom = c.Address
	//	}
	//}

	n.isCorrupted = isCorrupted
	n.addressToDropFrom = addressToDropFrom

	// Iterate over the checkpoints in the start signal to record the expected nonces.
	for _, c := range start.Checkpoints {
		n.expectedNonces[c.Layer][c.Nonce] = true
		n.expectedNumCheckpoints[c.Layer]++
	}

	for i, _ := range n.onionsToSend {
		n.onionsToSend[i] = make([]queuedOnion, 0)
	}

	return true, nil
}

// Receive processes an incoming onion, decrypts it, and forwards it to the next hop.
func (n *Relay) Receive(oApi structs.OnionApi, timeReceived time.Time) error {
	n.wg.Wait() // Wait for the expected nonces to be recorded by startRun

	// Peel the onion to extract its contents, including the role, layer, and metadata.
	role, layer, metadata, peeled, nextHop, err := pi_t.PeelOnion(oApi.Onion, n.PrivateKey)
	if err != nil {
		return pl.WrapError(err, "relay.Receive(): failed to remove layer")
	}

	networkLatency := int64(utils.ConvertToFloat64(timeReceived)) - oApi.LastSentTimestamp
	metrics.SetLatencyBetweenHops(networkLatency, oApi.From, n.Address, layer)

	// If the relay is corrupted and the onion is from the specified client, drop the onion.
	if n.isCorrupted && oApi.From == n.addressToDropFrom {
		slog.Debug("Corrupted relay dropping onion from " + n.addressToDropFrom)
		return nil
	}

	wasBruised := false
	isCheckpoint := metadata.Nonce != ""

	// If the onion contains a nonce, it is a checkpoint.
	if isCheckpoint {
		isCheckpoint = true
		if _, present := n.expectedNonces[layer][metadata.Nonce]; present { // Verify the nonce.
			n.checkpointsReceived.GetAndSet(layer, func(i int) int {
				return i + 1
			})
			if role == onion_model.MIXER {
				slog.Debug("Mixer: Nonce was verified, dropping null block.")
				peeled.Sepal = peeled.Sepal.RemoveBlock() // Remove the null block from the onion.
			}
		} else { // If the nonce is not verified, add a bruise to the onion.
			if role == onion_model.MIXER {
				slog.Debug("Mixer: Nonce was not verified, dropping master key.")
				peeled.Sepal = peeled.Sepal.AddBruise()
				wasBruised = true
			}
		}

	} else if role == onion_model.MIXER {
		peeled.Sepal = peeled.Sepal.RemoveBlock() // If not a checkpoint, remove the block from the onion.
	}

	slog.Debug("Received onion", "ischeckpoint?", isCheckpoint, "layer", layer, "nextHop", nextHop, "role", role, "wasBruised", wasBruised, "isCorrupted", n.isCorrupted, "from", oApi.From)

	checkpointsReceivedThisLayer := n.checkpointsReceived.Get(layer)

	newQOnion := queuedOnion{onion: peeled, nextHop: nextHop, layer: layer, originallySent: oApi.OriginallySentTimestamp, timeReceived: timeReceived}
	n.mu.RLock()
	if layer < n.currentLayer { // onion is late
		n.mu.RUnlock()
		go n.sendOnion(newQOnion) // Forward the onion to the next hop.
	} else {
		n.mu.RUnlock()
		n.mu.Lock()
		n.onionsToSend[layer] = append(n.onionsToSend[layer], newQOnion)
		if checkpointsReceivedThisLayer >= int(config.GetTao()*float64(n.expectedNumCheckpoints[layer])) {
			for _, qOnion := range n.onionsToSend[layer] {
				go n.sendOnion(qOnion)
			}
			n.onionsToSend[layer] = make([]queuedOnion, 0)
		}
		n.mu.Unlock()
	}

	return nil
}

func (n *Relay) sendOnion(qOnion queuedOnion) {
	processingTime := utils.TimeSince(qOnion.timeReceived)
	metrics.SetProcessingTime(int64(processingTime), n.Address, qOnion.layer)

	if err := api_functions.SendOnion(qOnion.nextHop, n.Address, qOnion.originallySent, qOnion.onion, qOnion.layer); err != nil {
		slog.Error("Error sending onion", err)
	}
}

// RegisterWithBulletinBoard registers the relay with the bulletin board.
func (n *Relay) RegisterWithBulletinBoard() error {
	slog.Info("Sending relay registration request.", "id", n.ID)
	return n.updateBulletinBoard("/registerRelay", http.StatusCreated) // Register the relay with the expected status code.
}

// GetActiveNodes retrieves the list of active nodes from the bulletin board.
func (n *Relay) GetActiveNodes() ([]structs.PublicNodeApi, error) {
	// Construct the URL for the GET request to retrieve active nodes.
	url := fmt.Sprintf("%s/nodes", n.BulletinBoardUrl)
	// Send the GET request to the bulletin board.
	resp, err := http.Get(url)
	if err != nil {
		return nil, pl.WrapError(err, fmt.Sprintf("error making GET request to %s", url)) // Wrap and return any errors that occur during the GET request.
	}
	defer func(Body io.ReadCloser) {
		// Ensure the response body is closed to avoid resource leaks.
		if err2 := Body.Close(); err2 != nil {
			fmt.Printf("error closing response body: %v\n", err2)
		}
	}(resp.Body)

	// Check if the response status code indicates success.
	if resp.StatusCode != http.StatusOK {
		return nil, pl.NewError("unexpected status code: %d", resp.StatusCode) // Return an error if the status code is not OK.
	}

	var activeNodes []structs.PublicNodeApi // Declare a slice to hold the decoded list of active nodes.
	// Decode the response body into the activeNodes slice.
	if err = json.NewDecoder(resp.Body).Decode(&activeNodes); err != nil {
		return nil, pl.WrapError(err, "error decoding response body") // Wrap and return any errors that occur during decoding.
	}

	return activeNodes, nil // Return the list of active nodes.
}

// updateBulletinBoard updates the relay's information on the bulletin board.
func (n *Relay) updateBulletinBoard(endpoint string, expectedStatusCode int) error {
	n.mu.Lock()                  // Lock the mutex to ensure exclusive access to the relay's state during the update.
	defer n.mu.Unlock()          // Unlock the mutex when the function returns.
	t, _ := utils.GetTimestamp() // Record the current time for the update.

	// Marshal the relay's public information into JSON.
	if data, err := json.Marshal(structs.PublicNodeApi{
		ID:             n.ID,
		Address:        n.Address,
		PublicKey:      n.PublicKey,
		PrometheusPort: n.PrometheusPort,
		Host:           n.Host,
		Port:           n.Port,
		Time:           t,
	}); err != nil {
		return pl.WrapError(err, "relay.UpdateBulletinBoard(): failed to marshal relay info")
	} else {
		// Send a POST request to the bulletin board to update the relay's information.
		url := n.BulletinBoardUrl + endpoint
		if resp, err2 := http.Post(url, "application/json", bytes.NewBuffer(data)); err2 != nil {
			return pl.WrapError(err2, "relay.UpdateBulletinBoard(): failed to send POST request to bulletin board")
		} else {
			defer func(Body io.ReadCloser) {
				// Ensure the response body is closed to avoid resource leaks.
				if err3 := Body.Close(); err3 != nil {
					fmt.Printf("relay.UpdateBulletinBoard(): error closing response body: %v\n", err2)
				}
			}(resp.Body)
			// Check if the update was successful based on the HTTP status code.
			if resp.StatusCode != expectedStatusCode {
				return pl.NewError("failed to %s relay, status code: %d, %s", endpoint, resp.StatusCode, resp.Status)
			} else {
				n.lastUpdate = t // Update the last update timestamp.
			}
			return nil
		}
	}
}
