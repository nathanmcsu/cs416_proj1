/*
An ink miner that can be used in BlockArt

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"

	"./errorlib"
	"./shapelib"
)

//

////////////////////////////////////////////////////////////////////////////////////////////
// <TYPE DECLARATIONS>

// Represents the type of operation for a shape on the canvas
type OpType int

const (
	ADD OpType = iota
	REMOVE
)

type MinerResponse struct {
	Error   error
	Payload []interface{}
}

type MinerRequest struct {
	Payload []interface{}
}

type ArtnodeRequest struct {
	Token   string
	Payload []interface{}
}

// Settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32
	CanvasYMax uint32
}

// Settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string

	// The minimum number of ink miners that an ink miner should be
	// connected to. If the ink miner dips below this number, then
	// they have to retrieve more nodes from the server using
	// GetNodes().
	MinNumMinerConnections uint8

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32
	InkPerNoOpBlock uint32

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8
	PoWDifficultyNoOpBlock uint8

	// Canvas settings
	CanvasSettings CanvasSettings
}

// Used to send heartbeat to the server just shy of 1 second each beat
const TIME_BUFFER uint32 = 500

type Miner struct {
	logger                    *log.Logger
	localAddr                 net.Addr
	serverAddr                string
	serverConn                *rpc.Client
	miners                    map[string]*rpc.Client
	blockchain                map[string]*Block
	blockChildren             map[string][]string
	longestChainLastBlockHash string
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
	pubKeyString              string
	inkAccounts               map[string]uint32
	settings                  *MinerNetSettings
	nonces                    map[string]bool
	tokens                    map[string]bool
	newLongestChain           chan bool
	unminedOps                map[string]*OperationRecord
	unvalidatedOps            map[string]*OperationRecord
	validatedOps              map[string]*OperationRecord
	tempOps                   map[string]*OperationRecord
}

type Block struct {
	BlockNo      uint32
	PrevHash     string
	Records      []OperationRecord
	PubKeyString string
	Nonce        uint32
}

type Operation struct {
	Type        OpType
	Shape       shapelib.Shape
	InkCost     uint32
	ValidateNum uint8
	TimeStamp   int64
}

type OperationRecord struct {
	Op           Operation
	OpSig        string
	PubKeyString string
}

type Signature struct {
	R *big.Int
	S *big.Int
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

//

var (
	logger   *log.Logger
	alphabet = []rune("0123456789abcdef")
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&net.TCPAddr{})
	gob.Register([]Block{})
	gob.Register(Block{})
	gob.Register(Operation{})
	gob.Register(OperationRecord{})
	gob.Register(errorLib.InvalidBlockHashError(""))
	gob.Register(errorLib.DisconnectedError(""))
	gob.Register(errorLib.InvalidShapeSvgStringError(""))
	gob.Register(errorLib.ShapeSvgStringTooLongError(""))
	gob.Register(errorLib.InvalidShapeHashError(""))
	gob.Register(errorLib.ShapeOwnerError(""))
	gob.Register(errorLib.OutOfBoundsError{})
	gob.Register(errorLib.ShapeOverlapError(""))
	gob.Register(errorLib.InvalidShapeFillStrokeError(""))
	gob.Register(errorLib.InvalidSignatureError{})
	gob.Register(errorLib.InvalidTokenError(""))
	gob.Register(errorLib.ValidationError(""))
	miner := new(Miner)
	miner.init()
	miner.listenRPC()
	miner.registerWithServer()
	miner.getMiners()
	miner.getLongestChain()
	logger.SetPrefix("[Mining]\n")
	// go miner.testAddOperation() // UNCOMMENT to test op mining - can remove when ops start flowing
	for {
		miner.mineBlock()
	}
}

//

////////////////////////////////////////////////////////////////////////////////////////////
// <PRIVATE METHODS : MINER>

func (m *Miner) init() {
	args := os.Args[1:]
	m.serverAddr = args[0]
	m.blockchain = make(map[string]*Block)
	m.blockChildren = make(map[string][]string)
	m.nonces = make(map[string]bool)
	m.tokens = make(map[string]bool)
	m.miners = make(map[string]*rpc.Client)
	m.inkAccounts = make(map[string]uint32)
	m.unminedOps = make(map[string]*OperationRecord)
	m.unvalidatedOps = make(map[string]*OperationRecord)
	m.validatedOps = make(map[string]*OperationRecord)
	m.tempOps = make(map[string]*OperationRecord)
	m.inkAccounts[m.pubKeyString] = 0
	if len(args) <= 1 {
		logger.Fatalln("Missing keys, please generate with: go run generateKeys.go")
		// Can uncomment for lazy generate, just uncomment the bottom chunk
		// priv := generateNewKeys()
		// m.privKey = priv
		// m.pubKey = priv.PublicKey
	}
	// Proper Key Generate
	privBytes, _ := hex.DecodeString(args[2])
	//pubBytes, _ := hex.DecodeString(args[2])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Private Key")
	}

	pubKey := decodeStringPubKey(args[1])
	// pubKey, err := x509.ParsePKIXPublicKey(pubBytes)
	// if checkError(err) != nil {
	// 	log.Fatalln("Error with Public Key")
	// }

	// Verify if keys are correct
	data := []byte("Hello World")
	r, s, _ := ecdsa.Sign(rand.Reader, privKey, data)
	if !ecdsa.Verify(pubKey, data, r, s) {
		logger.Fatalln("Keys don't match, try again")
	} else {
		logger.Println("Keys are correct and verified")
	}

	m.privKey = *privKey
	m.pubKey = *pubKey
	m.pubKeyString = args[1]
	// End of Proper Key Generation

	m.newLongestChain = make(chan bool)
}

func (m *Miner) listenRPC() {
	addrs, _ := net.InterfaceAddrs()
	var externalIP string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				externalIP = ipnet.IP.String()
			}
		}
	}
	externalIP = externalIP + ":0"
	tcpAddr, err := net.ResolveTCPAddr("tcp", externalIP)
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)
	rpc.Register(m)
	m.localAddr = listener.Addr()
	logger.Println("Listening on: ", listener.Addr().String())
	go func() {
		for {
			conn, err := listener.Accept()
			checkError(err)
			logger.Println("New connection!")
			go rpc.ServeConn(conn)
		}
	}()
}

// Ink miner registers their address and public key to the server and starts sending heartbeats
func (m *Miner) registerWithServer() {
	serverConn, err := rpc.Dial("tcp", m.serverAddr)
	if checkError(err) != nil {
		log.Fatal("Server is not reachable")
	}
	settings := new(MinerNetSettings)
	err = serverConn.Call("RServer.Register", &MinerInfo{m.localAddr, m.pubKey}, settings)
	if checkError(err) != nil {
		//TODO: Crashing for now, will need to revisit if there is any softer way to handle the error
		log.Fatal("Couldn't Register to Server")
	}
	m.serverConn = serverConn
	m.settings = settings
	go m.startHeartBeats()
}

// Sends heartbeats every half second to the server to maintain connection
func (m *Miner) startHeartBeats() {
	var ignored bool
	m.serverConn.Call("RServer.HeartBeat", m.pubKey, &ignored)
	for {
		time.Sleep(time.Duration(m.settings.HeartBeat-TIME_BUFFER) * time.Millisecond)
		m.serverConn.Call("RServer.HeartBeat", m.pubKey, &ignored)
	}
}

// Gets miners from server if below MinNumMinerConnections
func (m *Miner) getMiners() {
	var addrSet []net.Addr
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if !isConnected {
			delete(m.miners, minerAddr)
		}
	}
	if len(m.miners) < int(m.settings.MinNumMinerConnections) {
		m.serverConn.Call("RServer.GetNodes", m.pubKey, &addrSet)
		m.connectToMiners(addrSet)
		logger.Println("Current set of Addresses and Miners: ", addrSet, m.miners)
	}
}

// Establishes RPC connections with miners in addrs array
func (m *Miner) connectToMiners(addrs []net.Addr) {
	for _, minerAddr := range addrs {
		if m.miners[minerAddr.String()] == nil {
			minerConn, err := rpc.Dial("tcp", minerAddr.String())
			if err != nil {
				log.Println(err)
				delete(m.miners, minerAddr.String())
			} else {
				m.miners[minerAddr.String()] = minerConn
				response := new(MinerResponse)
				request := new(MinerRequest)
				request.Payload = make([]interface{}, 1)
				request.Payload[0] = m.localAddr.String()
				minerConn.Call("Miner.BidirectionalSetup", request, response)
			}
		}
	}
}

// When a new miner joins the network, it'll ask all the neighbouring miners for their longest chain
// After retrieving the chain, it'll use one of them as it's starting chain
// This method will do the following:
//	After returning with a chain
// 	- Validate the shape with existing miner states (ink, exisiting shapes)
//	- Apply the Block's state to the miner to validate future blocks
// 	- Revert the blocks to earse the memory

// After the checks, it'll keep the current longest valid chain
// The new miner will then apply the blocks again and start mining from the end of that chain

func (m *Miner) getLongestChain() {
	request := new(MinerRequest)
	var currLongestHash string
	var currLongestChain []Block

	for _, minerCon := range m.miners {
		m.blockchain[m.settings.GenesisBlockHash] = &Block{0, "", []OperationRecord{}, m.pubKeyString, 0}
		singleResponse := new(MinerResponse)
		minerCon.Call("Miner.GetBlockChain", request, singleResponse)
		if len(singleResponse.Payload) > 1 {
			longestHash := singleResponse.Payload[0].(string)
			longestChain := singleResponse.Payload[1].([]Block)
			isBlockValid := true
			var chainlength int
			for i := len(longestChain) - 1; i >= 0; i-- {
				// Should be Genesis Block at longestChain[len(longestChain) -1 ]
				// And last added block at index 0
				block := longestChain[i]
				opRecs := block.Records
				isOpValid := true
				for j := 0; j < len(opRecs); j++ {
					// check if each operation is a valid shape
					_, err := m.validateNewShape(opRecs[j].Op.Shape)
					if err != nil {
						// If error break and set flag
						isOpValid = false
						break
					}
					m.validatedOps[opRecs[j].OpSig] = &opRecs[j]
				}

				var myHash string
				if i == 0 {
					myHash = longestHash
				} else {
					myHash = longestChain[i-1].PrevHash
				}
				// If an operation of a block was invalid or the blockHash is invalid, the block is invalid.
				// Set flag, keep track of how long the chain was before invalid, and break.
				//
				// TODO: There seems to be some repeated/redundant work here that we should
				// figure out if we get the chance.
				if !isOpValid || m.validateBlock(&block, myHash) != nil {
					isBlockValid = false
					chainlength = i
					break
				}
				// else op is valid, apply the block to simulate
				m.applyBlockInk(&block)
				m.blockchain[myHash] = &block
			}
			// Chain is not valid all the way through
			if !isBlockValid {
				// Save the hash of the "new" end
				// slice it at length of chain - chainlength (where we broke) to get the last valid block of the chain
				longestHash = longestChain[len(longestChain)-chainlength-1].PrevHash
				longestChain = longestChain[len(longestChain)-chainlength:]
			}

			// Revert states
			m.validatedOps = make(map[string]*OperationRecord)
			m.inkAccounts = make(map[string]uint32)
			m.blockchain = make(map[string]*Block)

			// Set tracker if we have a new VALID longestChain & hash
			if len(currLongestHash) < 1 {
				currLongestChain = longestChain
				currLongestHash = longestHash
			} else if len(longestChain) > len(currLongestChain) {
				currLongestChain = longestChain
				currLongestHash = longestHash
			}
		}
	}
	if len(currLongestHash) > 1 {
		longestBlockHash := currLongestHash
		longestBlockChain := currLongestChain
		currHash := longestBlockHash
		for i := 0; i < len(longestBlockChain); i++ {
			// Should be from Latest block to Earliest/Genesis
			// Assumptions:
			//	-is that the chain was validated earlier ^, just need to apply the blocks and ops now
			block := &longestBlockChain[i]
			m.applyBlockInk(block)
			m.blockchain[currHash] = block
			m.addBlockChild(block, currHash)
			for _, opRec := range block.Records {
				if m.isOpValidateNumFulfilled(longestBlockHash, &opRec, block) {
					m.validatedOps[opRec.OpSig] = &opRec
				} else {
					m.unvalidatedOps[opRec.OpSig] = &opRec
				}
			}
			currHash = longestBlockChain[i].PrevHash
		}
		m.longestChainLastBlockHash = longestBlockHash
		logger.Println("Got an existing chain, start mining at blockNo: ", m.blockchain[m.longestChainLastBlockHash].BlockNo+1)
	}
	m.blockchain[m.settings.GenesisBlockHash] = &Block{0, "", []OperationRecord{}, m.pubKeyString, 0}

}

// Creates a block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func (m *Miner) mineBlock() {
	var nonce uint32 = 0
	var prevHash string
	var blockNo uint32

	if m.longestChainLastBlockHash == "" {
		prevHash = m.settings.GenesisBlockHash
		blockNo = 1
	} else {
		prevHash = m.longestChainLastBlockHash
		blockNo = m.blockchain[prevHash].BlockNo + 1
	}
	for {
		select {
		case <-m.newLongestChain:
			logger.Println("Got a new longest chain, switching to: ", m.longestChainLastBlockHash)
			return
		default:
			var block Block
			// Will create a opBlock or noOpBlock depending upon whether unminedOps are waiting to be mined
			if len(m.unminedOps) > 0 {
				var opRecordArray []OperationRecord
				for _, opRecord := range m.unminedOps {
					opRecordArray = append(opRecordArray, *opRecord)
				}
				block = Block{blockNo, prevHash, opRecordArray, m.pubKeyString, nonce}
			} else {
				block = Block{blockNo, prevHash, nil, m.pubKeyString, nonce}
			}
			if m.blockSuccessfullyMined(&block) {
				return
			} else {
				nonce++
			}
		}
	}
}

// Subtracts and credits ink to the ink accounts of each operation owner
// within a specified block, as well as ink for the mined block itself.
//
// TODO: Use a mutex
//
func (m *Miner) applyBlockInk(block *Block) {
	// update ink per operation
	for _, record := range block.Records {
		op := record.Op
		if _, exists := m.inkAccounts[record.PubKeyString]; !exists {
			m.inkAccounts[record.PubKeyString] = 0
		}
		if op.Type == ADD {
			m.inkAccounts[record.PubKeyString] -= op.InkCost
		} else {
			m.inkAccounts[record.PubKeyString] += op.InkCost
		}
	}

	// add ink for the newly mined block
	if _, exists := m.inkAccounts[block.PubKeyString]; !exists {
		m.inkAccounts[block.PubKeyString] = 0
	}
	if len(block.Records) == 0 {
		m.inkAccounts[block.PubKeyString] += m.settings.InkPerNoOpBlock
	} else {
		m.inkAccounts[block.PubKeyString] += m.settings.InkPerOpBlock
	}
}

func (m *Miner) isOpValidateNumFulfilled(headBlockHash string, opRecord *OperationRecord, opBlock *Block) bool {
	headBlockNo := m.blockchain[headBlockHash].BlockNo
	blockNo := opBlock.BlockNo
	return headBlockNo-blockNo >= uint32(opRecord.Op.ValidateNum)
}

// Manages miner state updates during a fast-forward OR a branch switch.
//
// In the case of a branch switch, we perform the following procedure:
// - Traverse the blocks in the old branch one at a time, up to the most
//   recent common ancestor
//     - Update (reverse) ink accounts for each block
//     - In each block, for each operation:
//         If it is in the unvalidated group, then see if it exists in the new branch
//           If it exists in the new branch:
//             If its validateNum has been satisfied, then move it to the validated group
//             If its validateNum has not been satisfied, then keep it in the unvalidated group
//           If it doesn't exist in the new branch, move it back to the unmined group
//         If it isn't in the unvalidated group (and is therefore in the validated group),
//         then remove it from the validated group and discard it
//
// Assumption: oldBlockHash and newBlockHash must both be valid block hashes
// for blocks which exist in the miner's current block map, and are both
// connected to the genesis block.
//
// TODO: Mutex
//
func (m *Miner) changeBlockchainHead(oldBlockHash, newBlockHash string) {
	// newBlock and oldBlock are "current" block pointers in the new and
	// old blockchain, respectively, as we traverse backwards
	newBlock := m.blockchain[newBlockHash]
	oldBlock := m.blockchain[oldBlockHash]
	newBranch := []*Block{}
	oldBranch := []*Block{}
	newBranchOps := make(map[string]bool)

	// [Fast Forward + Branch Switch]
	// Add blocks to the new branch up to the block num of the old branch head
	for newBlock.BlockNo > oldBlock.BlockNo {
		newBranch = append(newBranch, newBlock)
		for _, opRecord := range newBlock.Records {
			newBranchOps[opRecord.OpSig] = true
		}
		newBlock = m.blockchain[newBlock.PrevHash]
	}

	// [Branch Switch Only]
	// At this point, if newBlock and oldBlock are not equal, then we need to
	// perform a branch switch. We add blocks to the new and old branches up to
	// the most recent common ancestor.
	for newBlock != oldBlock {
		newBranch = append(newBranch, newBlock)
		for _, opRecord := range newBlock.Records {
			newBranchOps[opRecord.OpSig] = true
		}
		oldBranch = append(oldBranch, oldBlock)
		newBlock = m.blockchain[newBlock.PrevHash]
		oldBlock = m.blockchain[oldBlock.PrevHash]
	}

	// [Branch Switch Only]
	// Take each operation in the old branch and check against operations in
	// the new branch to take appropriate action.
	for _, block := range oldBranch {
		for _, opRecord := range block.Records {
			if _, opUnvalidated := m.unvalidatedOps[opRecord.OpSig]; opUnvalidated {
				if _, inNewBranch := newBranchOps[opRecord.OpSig]; inNewBranch {
					// Remove from unvalidated (it will be dealt with later)
					delete(m.unvalidatedOps, opRecord.OpSig)
				} else {
					// Move from unvalidated to unmined
					m.unminedOps[opRecord.OpSig] = &opRecord
					delete(m.unvalidatedOps, opRecord.OpSig)
				}
			} else {
				// Reverse ink account for the op and remove from validated
				op := opRecord.Op
				if op.Type == ADD {
					m.inkAccounts[opRecord.PubKeyString] += op.InkCost
				} else {
					m.inkAccounts[opRecord.PubKeyString] -= op.InkCost
				}
				delete(m.validatedOps, opRecord.OpSig)
			}
		}
		// Reverse ink account for the mined block
		if len(block.Records) == 0 {
			m.inkAccounts[block.PubKeyString] -= m.settings.InkPerNoOpBlock
		} else {
			m.inkAccounts[block.PubKeyString] -= m.settings.InkPerOpBlock
		}
	}

	// [Fast Forward + Branch Switch]
	// Take each operation in the new branch and either add it to validated
	// or unvalidated. Update ink accounts.
	for _, block := range newBranch {
		for _, opRecord := range block.Records {
			if m.isOpValidateNumFulfilled(newBlockHash, &opRecord, block) {
				m.validatedOps[opRecord.OpSig] = &opRecord
			} else {
				m.unvalidatedOps[opRecord.OpSig] = &opRecord
			}
		}
		m.applyBlockInk(block)
	}

	m.longestChainLastBlockHash = newBlockHash
	m.newLongestChain <- true
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateToConnectedMiners(block *Block, blockHash string) error {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 2)
	request.Payload[0] = *block
	request.Payload[1] = blockHash
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			err := minerCon.Call("Miner.SendBlock", request, response)
			checkError(err)
		} else {
			delete(m.miners, minerAddr)
		}
	}
	return nil
}

// Adds a block's hash to its parent's list of child hashes.
//
func (m *Miner) addBlockChild(block *Block, hash string) {
	if _, exists := m.blockChildren[block.PrevHash]; !exists {
		m.blockChildren[block.PrevHash] = []string{hash}
	} else {
		children := m.blockChildren[block.PrevHash]
		m.blockChildren[block.PrevHash] = append(children, hash)
	}
}

func (m *Miner) validateNewShape(s shapelib.Shape) (inkCost uint32, err error) {
	canvasSettings := m.settings.CanvasSettings
	_, geo, err := s.IsValid(canvasSettings.CanvasXMax, canvasSettings.CanvasYMax)
	if err != nil {
		return
	} else if inkCost = uint32(geo.GetInkCost()); inkCost > m.inkAccounts[m.pubKeyString] {
		err = errorLib.InsufficientInkError(m.inkAccounts[m.pubKeyString])
		return
	} else {
		// Check against all unmined, unvalidated, and validated operations
		if overlaps, hash := m.hasOverlappingShape(s, geo); overlaps {
			err = errorLib.ShapeOverlapError(hash)
			return
		}
	}
	return
}

func (m *Miner) hasOverlappingShape(s shapelib.Shape, geo shapelib.ShapeGeometry) (overlaps bool, hash string) {
	for hash, opRecord := range m.unminedOps {
		_s := opRecord.Op.Shape
		if _s.Owner == s.Owner {
			continue
		} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
			return true, hash
		}
	}
	for hash, opRecord := range m.unvalidatedOps {
		_s := opRecord.Op.Shape
		if _s.Owner == s.Owner {
			continue
		} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
			return true, hash
		}
	}
	for hash, opRecord := range m.validatedOps {
		_s := opRecord.Op.Shape
		if _s.Owner == s.Owner {
			continue
		} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
			return true, hash
		}
	}
	for hash, opRecord := range m.tempOps {
		_s := opRecord.Op.Shape
		if _s.Owner == s.Owner {
			continue
		} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
			return true, hash
		}
	}
	return false, hash
}

func (m *Miner) blockSuccessfullyMined(block *Block) bool {
	encodedBlock, err := json.Marshal(*block)
	checkError(err)
	blockHash := md5Hash(encodedBlock)
	if m.hashMatchesPOWDifficulty(blockHash) {
		logger.Println("Found a new Block!: ", block, blockHash)
		m.blockchain[blockHash] = block
		m.addBlockChild(block, blockHash)
		logger.Println("Current BlockChainMap: ", m.blockchain)
		m.longestChainLastBlockHash = blockHash
		m.disseminateToConnectedMiners(block, blockHash)
		m.applyBlockInk(block) // should this happen here? or once validateNum comes in to effect?
		m.moveUnminedToUnvalidated(block)
		return true
	} else {
		return false
	}
}

// Asserts that block hash matches the intended POW difficulty
func (m *Miner) hashMatchesPOWDifficulty(blockhash string) bool {
	return strings.HasSuffix(blockhash, strings.Repeat("0", int(m.settings.PoWDifficultyNoOpBlock)))
}

func (m *Miner) moveUnminedToUnvalidated(block *Block) {
	for _, opRecord := range block.Records {
		m.unvalidatedOps[opRecord.OpSig] = &opRecord
		delete(m.unminedOps, opRecord.OpSig)
	}
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateOpToConnectedMiners(opRec *OperationRecord) {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = *opRec
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			minerCon.Call("Miner.SendOp", request, response)
		} else {
			delete(m.miners, minerAddr)
		}
	}
}

// </PRIVATE METHODS : MINER>
////////////////////////////////////////////////////////////////////////////////////////////

//

////////////////////////////////////////////////////////////////////////////////////////////
// <RPC METHODS>

func (m *Miner) Hello(_ string, nonce *string) error {
	*nonce = getRand256()
	m.nonces[*nonce] = true
	return nil
}

// Once a token is successfully retrieved, that nonce can no longer be used
//
func (m *Miner) GetToken(request *ArtnodeRequest, response *MinerResponse) (err error) {
	nonce := request.Payload[0].(string)
	r := new(big.Int)
	s := new(big.Int)
	r, r_ok := r.SetString(request.Payload[1].(string), 0)
	s, s_ok := s.SetString(request.Payload[2].(string), 0)

	if !r_ok || !s_ok {
		response.Error = new(errorLib.InvalidSignatureError)
		return
	}

	_, validNonce := m.nonces[nonce]
	validSignature := ecdsa.Verify(&m.pubKey, []byte(nonce), r, s)

	if validNonce && validSignature {
		delete(m.nonces, nonce)
		response.Error = nil
		response.Payload = make([]interface{}, 3)
		token := getRand256()
		m.tokens[token] = true

		response.Payload[0] = token
		response.Payload[1] = m.settings.CanvasSettings.CanvasXMax
		response.Payload[2] = m.settings.CanvasSettings.CanvasYMax
	} else {
		response.Error = new(errorLib.InvalidSignatureError)
	}

	return nil
}

// Gets the svg string for the shape identified by a given shape hash (operation
// signature), if it exists.
//
// This only checks for ops in the validated group (because there's no way an art
// app could get the hash of an unvalidated operation).
//
func (m *Miner) GetSvgString(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	opRecord := m.validatedOps[hash]
	if opRecord == nil {
		response.Error = errorLib.InvalidShapeHashError(hash)
		return nil
	}

	shape := opRecord.Op.Shape
	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = `<path d="` + shape.ShapeSvgString + `" stroke="` + shape.Stroke + `" fill="` + shape.Fill + `"/>`
	return nil
}

func (m *Miner) SendBlock(request *MinerRequest, response *MinerResponse) error {
	logger.Println("Received Block: ", request.Payload[1].(string))

	block := request.Payload[0].(Block)
	blockHash := request.Payload[1].(string)

	err := m.validateBlock(&block, blockHash)
	if err != nil {
		return err
	}

	m.moveUnminedToUnvalidated(&block)
	//		If Valid, add to block chain
	//		Else return invalid

	// If new block, disseminate
	if _, exists := m.blockchain[blockHash]; !exists {
		m.blockchain[blockHash] = &block
		m.addBlockChild(&block, blockHash)
		// compute longest chain
		newChainLength := m.lengthLongestChain(blockHash)
		oldChainLength := m.lengthLongestChain(m.longestChainLastBlockHash)
		logger.Println(newChainLength, oldChainLength)
		logger.Println("Block hash: ", blockHash)
		logger.Println("Longest Chain hash: ", m.longestChainLastBlockHash)
		if newChainLength > oldChainLength {
			if oldChainLength == 0 {
				m.changeBlockchainHead(m.settings.GenesisBlockHash, blockHash)
			} else {
				m.changeBlockchainHead(m.longestChainLastBlockHash, blockHash)
			}
		} else if newChainLength == oldChainLength {
			if blockHash > m.longestChainLastBlockHash {
				m.changeBlockchainHead(m.longestChainLastBlockHash, blockHash)
			}
		}

		//		Disseminate Block to connected Miners
		m.disseminateToConnectedMiners(&block, blockHash)
	}
	return nil
}

func (m *Miner) SendOp(request *MinerRequest, response *MinerResponse) error {
	opRec := request.Payload[0].(OperationRecord)
	logger.Println("Received Op: ", opRec.OpSig)
	isSigValid := m.validateOp(opRec)
	// If new block, disseminate
	_, unMinedExists := m.unminedOps[opRec.OpSig]
	_, unValidExists := m.unvalidatedOps[opRec.OpSig]
	_, validExists := m.validatedOps[opRec.OpSig]

	if !unMinedExists && !unValidExists && !validExists && isSigValid {
		m.unminedOps[opRec.OpSig] = &opRec

		//	Disseminate Op to connected Miners
		m.disseminateOpToConnectedMiners(&opRec)
	}
	return nil
}

// Pings all miners currently listed in the miner map
// If a connected miner fails to reply, that miner should be removed from the map
func (m *Miner) PingMiner(payload string, reply *bool) error {
	*reply = true
	return nil
}

func (m *Miner) BidirectionalSetup(request *MinerRequest, response *MinerResponse) error {
	minerAddr := request.Payload[0].(string)
	minerConn, err := rpc.Dial("tcp", minerAddr)
	if err != nil {
		delete(m.miners, minerAddr)
	} else {
		m.miners[minerAddr] = minerConn
		logger.Println("birectional setup complete")
	}
	return nil
}

func (m *Miner) GetBlockChain(request *MinerRequest, response *MinerResponse) error {
	logger.Println("GetBlockChain")
	if len(m.longestChainLastBlockHash) < 1 {
		return nil
	}

	longestChainLength := m.blockchain[m.longestChainLastBlockHash].BlockNo
	longestChain := make([]Block, longestChainLength)

	var currhash = m.longestChainLastBlockHash
	for i := 0; i < int(longestChainLength); i++ {
		longestChain[i] = *m.blockchain[currhash]
		currhash = m.blockchain[currhash].PrevHash
	}
	response.Error = nil
	response.Payload = make([]interface{}, 2)
	response.Payload[0] = m.longestChainLastBlockHash
	response.Payload[1] = longestChain

	return nil
}

// Get the amount of ink remaining associated with the miners pub/priv key pair
func (m *Miner) GetInk(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.inkAccounts[m.pubKeyString]

	return nil
}

// Get the hash of the genesis block
func (m *Miner) GetGenesisBlock(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.settings.GenesisBlockHash

	return nil
}

// Gets a list of shape hashes (operation signatures) in a given block.
//
func (m *Miner) GetShapes(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	block := m.blockchain[hash]
	if block == nil {
		response.Error = errorLib.InvalidBlockHashError(hash)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	shapeHashes := make([]string, len(block.Records))
	for i, record := range block.Records {
		shapeHashes[i] = record.OpSig
	}
	response.Payload[0] = shapeHashes

	return nil
}

// Get a list of block hashes which are children of a given block
func (m *Miner) GetChildren(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	children, exists := m.blockChildren[hash]
	if !exists {
		response.Error = errorLib.InvalidBlockHashError(hash)
		return nil
	}
	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = children

	return nil
}

func (m *Miner) AddShape(request *ArtnodeRequest, response *MinerResponse) (err error) {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	validateNum := request.Payload[0].(uint8)
	shapeType := shapelib.ShapeType(request.Payload[1].(int))
	shapeSvgString := request.Payload[2].(string)
	fill := strings.Trim(request.Payload[3].(string), " ")
	stroke := strings.Trim(request.Payload[4].(string), " ")

	shape := shapelib.Shape{
		ShapeType:      shapeType,
		ShapeSvgString: shapeSvgString,
		Fill:           fill,
		Stroke:         stroke,
		Owner:          m.pubKeyString}

	inkCost, err := m.validateNewShape(shape)
	if err != nil {
		response.Error = err
		return
	}

	op := Operation{
		Type:        ADD,
		Shape:       shape,
		InkCost:     inkCost,
		ValidateNum: validateNum,
		TimeStamp:   time.Now().UnixNano()}

	opSig := m.addOperationRecord(&op)

	response.Error = nil
	response.Payload = make([]interface{}, 2)
	response.Payload[0] = opSig
	response.Payload[1] = m.inkAccounts[m.pubKeyString] - inkCost

	return
}

func (m *Miner) DeleteShape(request *ArtnodeRequest, response *MinerResponse) (err error) {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	shapeHash := request.Payload[0].(string)
	validateNum := request.Payload[1].(uint8)

	opRecord := m.validatedOps[shapeHash]
	if opRecord == nil || opRecord.PubKeyString != m.pubKeyString {
		response.Error = errorLib.ShapeOwnerError(shapeHash)
		return
	}

	delShape := opRecord.Op.Shape
	delShape.Fill, delShape.Stroke = "white", "white"

	op := Operation{
		Type:        REMOVE,
		Shape:       delShape,
		InkCost:     0, // Set to 0, to further identify delete
		ValidateNum: validateNum,
		TimeStamp:   time.Now().UnixNano()}

	opSig := m.addOperationRecord(&op)

	response.Error = nil
	response.Payload = make([]interface{}, 2)
	response.Payload[0] = opSig
	response.Payload[1] = m.inkAccounts[m.pubKeyString] + opRecord.Op.InkCost

	return
}

func (m *Miner) OpValidated(request *ArtnodeRequest, response *MinerResponse) (err error) {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	opSig := request.Payload[0].(string)
	op := m.validatedOps[opSig]

	response.Payload = make([]interface{}, 2)
	response.Payload[0] = false
	response.Payload[1] = ""
	if op == nil {
		response.Payload[0] = false
	} else {
		blockHash, err := m.getOpBlockHash(opSig)
		if err != nil {
			response.Error = err
		} else {
			response.Payload[0] = true
			response.Payload[1] = blockHash
		}
	}

	return
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

//

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

func (m *Miner) addOperationRecord(op *Operation) (opSig string) {
	encodedOp, err := json.Marshal(*op)
	checkError(err)
	r, s, err := ecdsa.Sign(rand.Reader, &m.privKey, encodedOp)
	checkError(err)
	sig := Signature{r, s}
	encodedSig, err := json.Marshal(sig)
	checkError(err)
	opSig = string(encodedSig)

	opRecord := OperationRecord{
		Op:           *op,
		OpSig:        opSig,
		PubKeyString: m.pubKeyString}

	m.unminedOps[opSig] = &opRecord
	m.disseminateOpToConnectedMiners(&opRecord)

	return
}

// Counts the length of the block chain given a block hash
func (m *Miner) lengthLongestChain(blockhash string) int {
	var length int
	if len(blockhash) < 1 {
		return length
	}
	var currhash = blockhash
	for {
		length++
		prevBlockHash := m.blockchain[currhash].PrevHash
		if _, exists := m.blockchain[prevBlockHash]; exists {
			currhash = prevBlockHash
			if currhash == m.settings.GenesisBlockHash {
				return length
			}
		} else {
			// Case where the last block in this chain isn't the Genesis one
			return 0
		}
	}
	return length
}

// Asserts the following about a given block and blockHash:
// - blockhash matches POW difficulty and nonce is correct
// - the given block points to a valid hash in the blockchain
func (m *Miner) validateBlock(block *Block, blockHash string) error {
	encodedBlock, err := json.Marshal(*block)
	checkError(err)
	newBlockHash := md5Hash(encodedBlock)
	if m.hashMatchesPOWDifficulty(newBlockHash) && m.validateOpIntegrity(block) && blockHash == newBlockHash && m.blockchain[block.PrevHash] != nil {
		logger.Println("Received Block has been validated")
		return nil
	}
	logger.Println("PROBLEM WITH VALIDATION")
	return errorLib.ValidationError(blockHash)
}

// Helper function to assert that each op in a block is signed properly,
// shape is valid, and the public key has enough ink.
func (m *Miner) validateOpIntegrity(block *Block) bool {
	for _, opRecord := range block.Records {
		if m.validateSignature(opRecord) {
			// add op to tempOps to check for shape harmony (yes, harmony :))
			m.tempOps[opRecord.OpSig] = &opRecord
			_, err := m.validateNewShape(opRecord.Op.Shape)
			if err != nil {
				return false
			}
		} else {
			return false
		}
	}
	// cleanup all tempOps
	for k := range m.tempOps {
		delete(m.tempOps, k)
	}
	return true
}

func (m *Miner) validateSignature(opRecord OperationRecord) bool {
	data, _ := json.Marshal(opRecord.Op)
	sig := new(Signature)
	json.Unmarshal([]byte(opRecord.OpSig), &sig)
	return ecdsa.Verify(decodeStringPubKey(opRecord.PubKeyString), data, sig.R, sig.S)
}

// Asserts the following about a given OperationRecord:
// TODO: shape, ink and valid sig
func (m *Miner) validateOp(opRec OperationRecord) bool {
	return true
}

func (m *Miner) getOpBlockHash(opSig string) (string, error) {
	hash := m.longestChainLastBlockHash
	block := m.blockchain[hash]
	blockNo := block.BlockNo
	for blockNo > 1 {
		ops := block.Records
		for _, op := range ops {
			if op.OpSig == opSig {
				return hash, nil
			}
		}

		hash = block.PrevHash
		block = m.blockchain[hash]
		blockNo = block.BlockNo
	}

	return "", errorLib.InvalidShapeHashError(opSig)
}

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

// TODO: CLEANUP will not need to use this function when using keys from command line
func generateNewKeys() ecdsa.PrivateKey {
	c := elliptic.P521()
	privKey, err := ecdsa.GenerateKey(c, rand.Reader)
	checkError(err)
	return *privKey
}

func decodeStringPubKey(pubkey string) *ecdsa.PublicKey {
	pubBytes, _ := hex.DecodeString(pubkey)
	pubKey, err := x509.ParsePKIXPublicKey(pubBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Public Key")
	}
	return pubKey.(*ecdsa.PublicKey)
}

// Generates a secure 256-bit nonce/token string for
// artnode request authentication.
//
func getRand256() string {
	str := make([]rune, 64)
	maxIndex := big.NewInt(int64(len(alphabet)))
	for i := range str {
		index, _ := rand.Int(rand.Reader, maxIndex)
		str[i] = alphabet[index.Int64()]
	}
	return string(str)
}

// Computes the md5 hash of a given byte slice
func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

// UNCOMMENT to test op mining - can remove once ops begin to flow

// func (m *Miner) testAddOperation() {
// 	shape := &Shape{PATH, "svgString", "fill", "stroke", m.pubKey, make([]Command, 1), make([]Point, 1), make([]LineSegment, 1), Point{0, 1}, Point{1, 0}}
// 	op := &Operation{ADD, *shape, "shapehashstring", uint32(20), uint8(3)}
// 	opRecord := &OperationRecord{*op, "some sig", "somekey"}
// 	time.Sleep(time.Second * 5)
// 	m.unminedOps[opRecord.OpSig] = *opRecord
// 	return
// }

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
