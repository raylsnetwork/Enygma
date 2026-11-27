package core

import (
	"fmt"
	"math/big"

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

// MerkleTree represents a sparse Merkle tree with Poseidon hash
type MerkleTree struct {
	depth      int
	zeros      []*big.Int
	tree       [][]*big.Int
	treeNumber int
	prevTrees  [][][]*big.Int
	// exclusiveCapacity selects the rollover-trigger convention this tree's
	// InsertLeaves uses, which MUST match whichever on-chain vault contract
	// this tree is tracking — the two vault contract families in this
	// monorepo disagree:
	//   - false (default, NewMerkleTree): matches enygma_dvp's own
	//     Merkle.sol (`(nextLeafIndex+count) >= 2**treeDepth` triggers
	//     roll) — used by Erc20CoinVault/Erc721CoinVault/Erc1155CoinVault/
	//     EnygmaErc20CoinVault and everything built on them (including the
	//     Payment/USDr vaults), in both enygma_dvp and enygma_retail_payments
	//     (which deploys enygma_dvp's exact compiled bytecode).
	//   - true (NewMerkleTreeStrict): matches enygma_dvp_auctions'
	//     AuctionCoinVault.sol (`(nextLeafIndex+count) > 2**treeDepth`
	//     triggers roll — the mathematically tight boundary, filling a
	//     tree completely before rolling instead of one leaf early).
	// Using the wrong one produces a local root/TreeNumber that silently
	// diverges from the on-chain vault as soon as a tree crosses depth-1
	// leaves — see the MerkleTree off-by-one bug fixed 2026-09-14.
	exclusiveCapacity bool
}

// NewMerkleTree creates a new Merkle tree with the given depth, using the
// rollover convention that matches enygma_dvp's own Merkle.sol (and every
// vault built on it, in both enygma_dvp and enygma_retail_payments). For a
// tree tracking an enygma_dvp_auctions AuctionCoinVault, use
// NewMerkleTreeStrict instead — see MerkleTree.exclusiveCapacity.
func NewMerkleTree(depth int) *MerkleTree {
	return newMerkleTree(depth, false)
}

// NewMerkleTreeStrict creates a new Merkle tree using the tight rollover
// boundary that matches enygma_dvp_auctions' AuctionCoinVault.sol (rolls
// only once a tree is completely full, not one leaf early). See
// MerkleTree.exclusiveCapacity for why this must match the specific vault
// contract the tree is tracking.
func NewMerkleTreeStrict(depth int) *MerkleTree {
	return newMerkleTree(depth, true)
}

func newMerkleTree(depth int, exclusiveCapacity bool) *MerkleTree {
	mt := &MerkleTree{
		depth:             depth,
		treeNumber:        0,
		prevTrees:         make([][][]*big.Int, 0),
		exclusiveCapacity: exclusiveCapacity,
	}

	mt.zeros = getZeroValueLevels(depth)
	mt.tree = make([][]*big.Int, depth+1)
	for i := 0; i <= depth; i++ {
		mt.tree[i] = make([]*big.Int, 0)
	}
	mt.tree[depth] = []*big.Int{HashLeftRight(mt.zeros[depth-1], mt.zeros[depth-1])}

	return mt
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

// InsertLeaves inserts leaves as one batch, as one call of the vault's
// insertLeaves does: a batch that would fill the current tree goes whole
// into the next one. A vault decides this per call, and one transaction can
// make several calls, so a tree rebuilt from a vault's events places each
// leaf with InsertAt, at the position its LeafInserted event names.
//
// It takes the batches Merkle.sol takes. An empty batch, which the vault
// refuses (EmptyLeafBatch), changes nothing. A batch of 2^depth leaves or
// more, which it refuses too (LeafBatchTooLarge), panics: put whole in a
// fresh tree, it would fill the last slot or leave leaves out of the root.
// A NewMerkleTreeStrict tree follows AuctionCoinVault.sol, which has no
// such bound.
func (mt *MerkleTree) InsertLeaves(leaves []*big.Int) {
	if len(leaves) == 0 {
		return
	}
	maxLeaves := 1 << mt.depth
	if !mt.exclusiveCapacity && len(leaves) >= maxLeaves {
		panic(fmt.Sprintf("InsertLeaves: %d leaves in one batch, which a vault refuses: a tree of depth %d takes at most %d per insert", len(leaves), mt.depth, maxLeaves-1))
	}
	// Check if tree is full — see MerkleTree.exclusiveCapacity for why this
	// must match the on-chain vault contract this tree is tracking:
	//   - exclusiveCapacity=false (default): rolls at `(count+count) >= maxLeaves`,
	//     matching Merkle.sol (enygma_dvp's own vaults).
	//   - exclusiveCapacity=true (NewMerkleTreeStrict): rolls at
	//     `(count+count) > maxLeaves`, matching AuctionCoinVault.sol — a tree
	//     holding exactly maxLeaves leaves does not roll until the next
	//     insert would actually exceed capacity.
	full := len(mt.tree[0]) + len(leaves)
	rolls := full > maxLeaves
	if !mt.exclusiveCapacity {
		rolls = full >= maxLeaves
	}
	if rolls {
		mt.newTree()
	}

	mt.tree[0] = append(mt.tree[0], leaves...)

	// Rebuild tree
	mt.rebuildSparseTree()
}

// InsertLeaf inserts a single leaf into the tree, as a batch of one.
func (mt *MerkleTree) InsertLeaf(leaf *big.Int) {
	mt.InsertLeaves([]*big.Int{leaf})
}

// InsertAt puts leaf at index index of tree tree, where a vault's
// LeafInserted event says the vault put it. Replayed in chain order, each
// leaf takes the current tree's next index, or index 0 of the next tree
// when the vault started one for its batch. InsertAt refuses any other
// position, as a missed, repeated or reordered event gives, and an index
// past the tree's capacity, and leaves the tree as it was: a rebuild that
// drifted from the vault fails here, not in a proof the vault refuses.
func (mt *MerkleTree) InsertAt(tree, index int, leaf *big.Int) error {
	next := len(mt.tree[0])
	switch {
	case tree == mt.treeNumber && index == next:
	case tree == mt.treeNumber+1 && index == 0:
	default:
		return fmt.Errorf("leaf %s at tree %d index %d: the tree rebuilt so far takes tree %d index %d, or tree %d index 0",
			leaf, tree, index, mt.treeNumber, next, mt.treeNumber+1)
	}
	if index >= 1<<mt.depth {
		return fmt.Errorf("leaf %s at tree %d index %d: a tree of depth %d holds %d leaves", leaf, tree, index, mt.depth, 1<<mt.depth)
	}
	if tree != mt.treeNumber {
		mt.newTree()
	}
	mt.appendLeaf(leaf)
	return nil
}

// appendLeaf adds leaf as the current tree's next leaf and rehashes the
// nodes on its path to the root, the only ones it changes.
func (mt *MerkleTree) appendLeaf(leaf *big.Int) {
	mt.tree[0] = append(mt.tree[0], leaf)
	index := len(mt.tree[0]) - 1
	for level := 0; level < mt.depth; level++ {
		left := index &^ 1
		right := mt.zeros[level]
		if left+1 < len(mt.tree[level]) {
			right = mt.tree[level][left+1]
		}
		node := HashLeftRight(mt.tree[level][left], right)
		parent := index >> 1
		if parent < len(mt.tree[level+1]) {
			mt.tree[level+1][parent] = node
		} else {
			mt.tree[level+1] = append(mt.tree[level+1], node)
		}
		index = parent
	}
}

// newTree creates a new tree when the current one is full
func (mt *MerkleTree) newTree() {
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

// GenerateProof generates a Merkle proof for an element: its first copy in
// the current tree, else in the oldest earlier tree holding it. A vault
// takes the same commitment twice as two notes, each with its own position
// and nullifier; GenerateProofAt proves the one at a given position.
func (mt *MerkleTree) GenerateProof(element *big.Int) (*MerkleProof, error) {
	for i, leaf := range mt.tree[0] {
		if leaf.Cmp(element) == 0 {
			return mt.GenerateProofAt(mt.treeNumber, i)
		}
	}
	for t, prevTree := range mt.prevTrees {
		for i, leaf := range prevTree[0] {
			if leaf.Cmp(element) == 0 {
				return mt.GenerateProofAt(t, i)
			}
		}
	}
	return nil, fmt.Errorf("couldn't find %s in the MerkleTree number: %d", element.String(), mt.treeNumber)
}

// GenerateProofAt generates the Merkle proof of the leaf at index index of
// tree tree (an earlier tree's or the current one's), the position a
// vault's LeafInserted event gives a note.
func (mt *MerkleTree) GenerateProofAt(tree, index int) (*MerkleProof, error) {
	var levels [][]*big.Int
	switch {
	case tree == mt.treeNumber:
		levels = mt.tree
	case tree >= 0 && tree < len(mt.prevTrees):
		levels = mt.prevTrees[tree]
	default:
		return nil, fmt.Errorf("no tree %d: the trees are 0 to %d", tree, mt.treeNumber)
	}
	if index < 0 || index >= len(levels[0]) {
		return nil, fmt.Errorf("no leaf %d in tree %d, which holds %d", index, tree, len(levels[0]))
	}
	element := levels[0][index]

	// Each level's sibling, and the leaf index's bit there: 1 when the
	// node is a right child, its sibling on the left.
	elements := make([]*big.Int, 0, mt.depth)
	indicesBigInt := big.NewInt(0)
	for level := 0; level < mt.depth; level++ {
		if index%2 == 0 {
			right := mt.zeros[level]
			if index+1 < len(levels[level]) {
				right = levels[level][index+1]
			}
			elements = append(elements, right)
		} else {
			elements = append(elements, levels[level][index-1])
			indicesBigInt.SetBit(indicesBigInt, level, 1)
		}
		index = index / 2
	}

	return &MerkleProof{
		Element:    element,
		Elements:   elements,
		Indices:    indicesBigInt,
		Root:       levels[mt.depth][0],
		TreeNumber: tree,
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
