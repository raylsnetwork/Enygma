package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/sha3"
)

// SNARK_SCALAR_FIELD is the field modulus for the BN254 curve
var SNARK_SCALAR_FIELD, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// MerkleProof represents a proof of inclusion in the Merkle tree
type MerkleProof struct {
	Element  *big.Int
	Elements []*big.Int
	Indices  *big.Int
	Root     *big.Int
	// TreeNumber is the sub-tree the element was found in: 0 for the first
	// tree, incrementing once per rollover (see newTree/StTreeNumber) —
	// matches the numbering used on-chain for AuctionCoinVault.rootHistory
	// and similar per-vault root-history maps. Callers that submit a proof
	// on-chain for a tree that has since rolled over must pass this value,
	// not always 0.
	TreeNumber int
}

// MerkleTreeState represents the serializable state of a Merkle tree
type MerkleTreeState struct {
	Depth int        `json:"depth"`
	Tree  [][]string `json:"tree"`
	Zeros []string   `json:"zeros"`
}

// MerkleTree represents a sparse Merkle tree with Poseidon hash
type MerkleTree struct {
	depth      int
	zeros      []*big.Int
	tree       [][]*big.Int
	treeNumber int
	prevTrees  [][][]*big.Int
	savePath   string
}

// NewMerkleTree creates a new Merkle tree with the given depth
func NewMerkleTree(depth int) *MerkleTree {
	mt := &MerkleTree{
		depth:      depth,
		treeNumber: 0,
		prevTrees:  make([][][]*big.Int, 0),
	}

	mt.zeros = getZeroValueLevels(depth)
	mt.tree = make([][]*big.Int, depth+1)
	for i := 0; i <= depth; i++ {
		mt.tree[i] = make([]*big.Int, 0)
	}
	mt.tree[depth] = []*big.Int{HashLeftRight(mt.zeros[depth-1], mt.zeros[depth-1])}

	return mt
}

// NewMerkleTreeWithPath creates a new Merkle tree and loads from file if exists
func NewMerkleTreeWithPath(depth int, prefix string, clientPath string) (*MerkleTree, error) {
	mt := &MerkleTree{
		depth:      depth,
		treeNumber: 0,
		prevTrees:  make([][][]*big.Int, 0),
	}

	// Determine save path
	if clientPath != "" {
		mt.savePath = filepath.Join(clientPath, prefix+"MerkleTreeState.json")
	} else {
		mt.savePath = filepath.Join("./src/appdata/", prefix+"MerkleTreeState.json")
	}

	// Try to load from file
	if _, err := os.Stat(mt.savePath); err == nil {
		fmt.Printf("Tree file found at %s, loading...\n", mt.savePath)
		if err := mt.loadFromFile(); err != nil {
			return nil, err
		}
	} else {
		fmt.Println("No tree file found. Creating zero merkleTree")
		mt.zeros = getZeroValueLevels(depth)
		mt.tree = make([][]*big.Int, depth+1)
		for i := 0; i <= depth; i++ {
			mt.tree[i] = make([]*big.Int, 0)
		}
		mt.tree[depth] = []*big.Int{HashLeftRight(mt.zeros[depth-1], mt.zeros[depth-1])}
	}

	return mt, nil
}

// loadFromFile loads the Merkle tree state from a JSON file
func (mt *MerkleTree) loadFromFile() error {
	data, err := os.ReadFile(mt.savePath)
	if err != nil {
		return err
	}

	var state MerkleTreeState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	mt.depth = state.Depth

	// Parse zeros
	mt.zeros = make([]*big.Int, len(state.Zeros))
	for i, zeroStr := range state.Zeros {
		mt.zeros[i], _ = new(big.Int).SetString(zeroStr, 10)
	}

	// Parse tree
	mt.tree = make([][]*big.Int, len(state.Tree))
	for i, subtree := range state.Tree {
		mt.tree[i] = make([]*big.Int, len(subtree))
		for j, numStr := range subtree {
			mt.tree[i][j], _ = new(big.Int).SetString(numStr, 10)
		}
	}

	return nil
}

// SaveToFile saves the Merkle tree state to a JSON file
func (mt *MerkleTree) SaveToFile() error {
	// Ensure directory exists
	dirPath := filepath.Dir(mt.savePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	state := MerkleTreeState{
		Depth: mt.depth,
		Tree:  make([][]string, len(mt.tree)),
		Zeros: make([]string, len(mt.zeros)),
	}

	// Convert zeros to strings
	for i, zero := range mt.zeros {
		state.Zeros[i] = zero.String()
	}

	// Convert tree to strings
	for i, subtree := range mt.tree {
		state.Tree[i] = make([]string, len(subtree))
		for j, num := range subtree {
			state.Tree[i][j] = num.String()
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(mt.savePath, data, 0644); err != nil {
		return err
	}

	fmt.Printf("Tree has been saved to %s.\n", mt.savePath)
	return nil
}

// GetTree returns the tree state as a serializable object
func (mt *MerkleTree) GetTree() MerkleTreeState {
	state := MerkleTreeState{
		Depth: mt.depth,
		Tree:  make([][]string, len(mt.tree)),
		Zeros: make([]string, len(mt.zeros)),
	}

	for i, zero := range mt.zeros {
		state.Zeros[i] = zero.String()
	}

	for i, subtree := range mt.tree {
		state.Tree[i] = make([]string, len(subtree))
		for j, num := range subtree {
			state.Tree[i][j] = num.String()
		}
	}

	return state
}

// rebuildSparseTree rebuilds the tree from the leaves up
func (mt *MerkleTree) rebuildSparseTree() {
	for level := 0; level < mt.depth; level++ {
		mt.tree[level+1] = make([]*big.Int, 0)

		for pos := 0; pos < len(mt.tree[level]); pos += 2 {
			var right *big.Int
			if pos+1 < len(mt.tree[level]) {
				right = mt.tree[level][pos+1]
			} else {
				right = mt.zeros[level]
			}
			mt.tree[level+1] = append(mt.tree[level+1], HashLeftRight(mt.tree[level][pos], right))
		}
	}
}

// InsertLeaves inserts multiple leaves into the tree
func (mt *MerkleTree) InsertLeaves(leaves []*big.Int) {
	// Check if tree is full. Strictly-greater, matching AuctionCoinVault._insertLeaves'
	// on-chain rollover condition `(_nextLeafIndex + count) > (2 ** _treeDepth)` — a
	// tree holding exactly maxLeaves leaves does NOT roll until the next insert would
	// exceed capacity. Using >= here rolled one leaf early, so any local tree tracking
	// (e.g. for building Merkle proofs for a >=maxLeaves-leaf vault) diverged from the
	// on-chain root as soon as the tree passed maxLeaves-1 leaves.
	maxLeaves := 1 << mt.depth
	if len(mt.tree[0])+len(leaves) > maxLeaves {
		mt.newTree()
	}

	mt.tree[0] = append(mt.tree[0], leaves...)

	// Rebuild tree
	mt.rebuildSparseTree()
}

// InsertLeaf inserts a single leaf into the tree
func (mt *MerkleTree) InsertLeaf(leaf *big.Int) {
	mt.InsertLeaves([]*big.Int{leaf})
}

// newTree creates a new tree when the current one is full
func (mt *MerkleTree) newTree() {
	fmt.Println("MerkleTree is full. Going to the next tree.")

	// Save current tree to prevTrees
	treeCopy := make([][]*big.Int, len(mt.tree))
	for i, subtree := range mt.tree {
		treeCopy[i] = make([]*big.Int, len(subtree))
		copy(treeCopy[i], subtree)
	}
	mt.prevTrees = append(mt.prevTrees, treeCopy)
	mt.treeNumber++

	// Reset tree
	mt.zeros = getZeroValueLevels(mt.depth)
	mt.tree = make([][]*big.Int, mt.depth+1)
	for i := 0; i <= mt.depth; i++ {
		mt.tree[i] = make([]*big.Int, 0)
	}
	mt.tree[mt.depth] = []*big.Int{HashLeftRight(mt.zeros[mt.depth-1], mt.zeros[mt.depth-1])}
}

// GetLeaves returns all leaves in the current tree
func (mt *MerkleTree) GetLeaves() []*big.Int {
	return mt.tree[0]
}

// Root returns the root of the current tree
func (mt *MerkleTree) Root() *big.Int {
	return mt.tree[mt.depth][0]
}

// LastTreeNumber returns the number of previous trees
func (mt *MerkleTree) LastTreeNumber() int {
	return len(mt.prevTrees)
}

// RootOfPrevTree returns the root of a previous tree. treeNum is the 0-based
// index into prevTrees (0 = the first/oldest tree) — the same numbering
// GenerateProof uses when it falls back into prevTrees, and the numbering
// on-chain rootHistory maps use. treeNum=0 here means the first previous
// tree, not "the current tree" — the current tree's root is mt.Root().
func (mt *MerkleTree) RootOfPrevTree(treeNum int) *big.Int {
	return mt.prevTrees[treeNum][mt.depth][0]
}

// GenerateProof generates a Merkle proof for an element
func (mt *MerkleTree) GenerateProof(element *big.Int) (*MerkleProof, error) {
	// Initialize proof elements
	elements := make([]*big.Int, 0)
	indices := make([]int, 0)

	// Find element in current tree
	index := -1
	for i, leaf := range mt.tree[0] {
		if leaf.Cmp(element) == 0 {
			index = i
			break
		}
	}

	treeNum := -1
	if index == -1 {
		fmt.Println("merkle.GenerateProof: can not find in the current tree, looking into previous trees.")
		for i, prevTree := range mt.prevTrees {
			for j, leaf := range prevTree[0] {
				if leaf.Cmp(element) == 0 {
					index = j
					treeNum = i
					fmt.Printf("merkle.GenerateProof: found it in tree no %d\n", i)
					break
				}
			}
			if index != -1 {
				break
			}
		}
		if index == -1 {
			return nil, errors.New(fmt.Sprintf("Couldn't find %s in the MerkleTree number: %d", element.String(), mt.treeNumber))
		}
	}

	activeTree := mt.tree
	activeRoot := mt.Root()
	foundTreeNumber := mt.treeNumber
	if treeNum != -1 {
		activeTree = mt.prevTrees[treeNum]
		activeRoot = mt.RootOfPrevTree(treeNum)
		foundTreeNumber = treeNum
	}

	// Loop through each level
	for level := 0; level < mt.depth; level++ {
		if index%2 == 0 {
			// If index is even, get element on right
			var right *big.Int
			if index+1 < len(activeTree[level]) {
				right = activeTree[level][index+1]
			} else {
				right = mt.zeros[level]
			}
			elements = append(elements, right)
			indices = append(indices, 0)
		} else {
			// If index is odd, get element on left
			elements = append(elements, activeTree[level][index-1])
			indices = append(indices, 1)
		}

		// Get index for next level
		index = index / 2
	}

	// Convert indices to BigInt
	indicesBigInt := big.NewInt(0)
	for i := len(indices) - 1; i >= 0; i-- {
		indicesBigInt.Lsh(indicesBigInt, 1)
		if indices[i] == 1 {
			indicesBigInt.Or(indicesBigInt, big.NewInt(1))
		}
	}

	return &MerkleProof{
		Element:    element,
		Elements:   elements,
		Indices:    indicesBigInt,
		Root:       activeRoot,
		TreeNumber: foundTreeNumber,
	}, nil
}

// HashLeftRight computes the Poseidon hash of two elements
func HashLeftRight(left, right *big.Int) *big.Int {
	hash, err := poseidon.Hash([]*big.Int{left, right})
	if err != nil {
		panic(fmt.Sprintf("Poseidon hash failed: %v", err))
	}
	return hash
}

// GetZeroValue returns the zero value used for empty leaves
func GetZeroValue() *big.Int {
	// keccak256("ZkDvp") % SNARK_SCALAR_FIELD
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte("ZkDvp"))
	hash := hasher.Sum(nil)

	result := new(big.Int).SetBytes(hash)
	result.Mod(result, SNARK_SCALAR_FIELD)
	return result
}

// getZeroValueLevels computes the zero values for each level of the tree
func getZeroValueLevels(depth int) []*big.Int {
	levels := make([]*big.Int, depth)

	// First level is the leaf zero value
	levels[0] = GetZeroValue()

	// Loop through remaining levels
	for level := 1; level < depth; level++ {
		levels[level] = HashLeftRight(levels[level-1], levels[level-1])
	}

	return levels
}

// Depth returns the depth of the tree
func (mt *MerkleTree) Depth() int {
	return mt.depth
}

// TreeNumber returns the current tree number
func (mt *MerkleTree) TreeNumber() int {
	return mt.treeNumber
}
