const utils = require('./utils');
BigInt.prototype.toJSON = function() { return this.toString() }
const fs = require('fs');
const path = require('path');

const SAVE_PATH = "./src/appdata/";

class MerkleTree {

  // added clientPath, if not null the tree would be saved/loaded to/from the path
  constructor(depth, prefix, clientPath='') {

    this.treeNumber = 0;
    this.prevTrees = [];
    this.savePath = "";


    try {
      // console.log(clientPath);
      if(clientPath != ''){
        console.log("undefined clientPath");
        this.savePath = clientPath + prefix + "MerkleTreeState.json";
      }
      else{
        this.savePath = SAVE_PATH + prefix + "MerkleTreeState.json";
      }


      if (fs.existsSync(this.savePath)) {
        console.log(`Tree file found at ${this.savePath}, loading...`);
        
        const storedTree = JSON.parse(fs.readFileSync(this.savePath, 'utf8'));
        this.depth = storedTree["depth"];
        this.zeros = storedTree["zeros"].map(BigInt);
        this.tree = storedTree["tree"].map(subtree => subtree.map(BigInt));
        this.depth = storedTree["depth"];

        this.zeros = [];
        for (let zeroString of  storedTree["zeros"]) {
          this.zeros.push(BigInt(zeroString));
        }
        this.tree = [];
        for (let subtree of  storedTree["tree"]) {
          let sub = [];
          for(let numString of subtree){
            sub.push(BigInt(numString));
          }
          this.tree.push(sub);
        }
      } else {
        throw new Error(`No tree file found at ${this.savePath}.`);
      }
    } catch (error) {
      console.log(`No tree file found. Creating zero merkleTree`);
        this.depth = depth;
        this.zeros = MerkleTree.getZeroValueLevels(depth);
        this.tree = Array(depth)
          .fill(0)
          .map(() => []);
        this.tree[depth] = [MerkleTree.hashLeftRight(this.zeros[depth - 1], this.zeros[depth - 1])];
        this.treeNumber = 0;
        // this.prevTrees.push([... this.tree]);

    }

  }


  // deprecated function
  // TODO:: remove it safely
  loadFromFile(depth, prefix){

    try {
      const storedTree = require(SAVE_PATH + prefix + "MerkleTreeState.json");
      console.log(`tree file found in ${SAVE_PATH + prefix + "MerkleTreeState.json"}, loading...`);
        this.depth = storedTree["depth"];

        this.zeros = [];
        for (let zeroString of  storedTree["zeros"]) {
          this.zeros.push(BigInt(zeroString));
        }
        this.tree = [];
        for (let subtree of  storedTree["tree"]) {
          let sub = [];
          for(let numString of subtree){
            sub.push(BigInt(numString));
          }
          this.tree.push(sub);
        }

    } catch (error) {
      console.log(`No tree file found. Creating zero merkleTree`);
        this.depth = depth;
        this.zeros = MerkleTree.getZeroValueLevels(depth);
        this.tree = Array(depth)
          .fill(0)
          .map(() => []);
        this.tree[depth] = [MerkleTree.hashLeftRight(this.zeros[depth - 1], this.zeros[depth - 1])];
        this.treeNumber = 0;
        // this.prevTrees.push([... this.tree]);

    }
  }

  loadFromLocalStorage(localTree, depth){
    console.log("Loading merkleTree from localStorage")
    try {
        this.depth = localTree.depth;

        this.zeros = [];
        for (let zeroString of  localTree.zeros) {
          this.zeros.push(BigInt(zeroString));
        }
        this.tree = [];
        for (let subtree of  localTree.tree) {
          let sub = [];
          for(let numString of subtree){
            sub.push(BigInt(numString));
          }
          this.tree.push(sub);
        }

    } catch (error) {
      console.log(`No tree found in localStorage, creating`);
        this.depth = depth;
        this.zeros = MerkleTree.getZeroValueLevels(depth);
        this.tree = Array(depth)
          .fill(0)
          .map(() => []);
        this.tree[depth] = [MerkleTree.hashLeftRight(this.zeros[depth - 1], this.zeros[depth - 1])];
        this.treeNumber = 0;
        this.prevTrees.push([... this.tree]);

    }
  }

  rebuildSparseTree() {
    for (let level = 0; level < this.depth; level += 1) {
      this.tree[level + 1] = [];

      for (let pos = 0; pos < this.tree[level].length; pos += 2) {
        this.tree[level + 1].push(
          MerkleTree.hashLeftRight(
            this.tree[level][pos],
            this.tree[level][pos + 1] ?? this.zeros[level],
          ),
        );
      }
    }
  }

  insertLeaves(leaves) {

    leaves = leaves.map(BigInt);

    if(this.tree[0].length + leaves.length >= Math.pow(2, this.depth)){
      this.newTree();
    }

    this.tree[0] = this.tree[0].concat(leaves);

    // Rebuild tree
    this.rebuildSparseTree();
  }

  newTree() {
      console.log("MerkleTree is full. Going to the next tree.");
      this.prevTrees.push([... this.tree]);
      this.treeNumber++;

      this.zeros = MerkleTree.getZeroValueLevels(this.depth);
      this.tree = Array(this.depth)
        .fill(0)
        .map(() => []);
      this.tree[this.depth] = [MerkleTree.hashLeftRight(this.zeros[this.depth - 1], this.zeros[this.depth - 1])];
  }

  getLeaves() {
    return this.tree[0];
  }

  saveToFile(prefix){

    // Ensure directory exists
    const dirPath = path.dirname(this.savePath);

    if (!fs.existsSync(dirPath)) {
        fs.mkdirSync(dirPath, { recursive: true });
        console.log(`Directory created: ${dirPath}`);
    }

    var obj = {
      "depth": this.depth,
      "tree": this.tree,
      "zeros": this.zeros
    };

    utils.writeToJson(this.savePath, obj);


    console.log(`Tree has been saved to ${this.savePath} .`);

  }

  getTree(){

      var obj = {
        "depth": this.depth,
        "tree": this.tree,
        "zeros": this.zeros
      };

      return obj
  }

  generateProof(element) {
    // eslint-disable-next-line no-param-reassign
    element = BigInt(element);

    // Initialize of proof elements
    const elements = [];

    // Initialize indicies string (binary, will be parsed to bigint)
    const indices = [];

    // Get initial index
    let index = this.tree[0].indexOf(element);
    let treeNum = -1;
    if (index === -1) {
      console.log("merkle.generateProof: can not find in the current tree, looking into previous trees.");
      
      for (let i = 0;i < this.prevTrees.length; i++){
          index = this.prevTrees[i][0].indexOf(element);
          if(index != -1){
            treeNum = i;
            console.log("merkle.generateProof: found it in tree no " + i);
            break;
          }
      }
      if(index == -1){
        throw new Error(`Couldn't find ${element} in the MerkleTree number: ${this.treeNumber}`);
      }
    }

    let activeTree = this.tree;
    let activeRoot = this.root;
    if(treeNum != -1){
        activeTree = this.prevTrees[treeNum];
        activeRoot = this.rootOfPrevTree(treeNum);
    }

    // Loop through each level
    for (let level = 0; level < this.depth; level += 1) {
      if (index % 2 === 0) {
        // If index is even get element on right
        elements.push(activeTree[level][index + 1] ?? this.zeros[level]);

        // Push bit to indices
        indices.push('0');
      } else {
        // If index is odd get element on left
        elements.push(activeTree[level][index - 1]);

        // Push bit to indices
        indices.push('1');
      }

      // Get index for next level
      index = Math.floor(index / 2);
    }

    // console.log("MerkleProof. indices: ", indices);
    return {
      element,
      elements,
      indices: BigInt(`0b${indices.reverse().join('')}`),
      root: activeRoot,
    };
  }

  get root() {
    return this.tree[this.depth][0];
  }

  get lastTreeNumber() {
    return this.prevTrees.length;
  }


  rootOfPrevTree(treeNum){
    if(treeNum == 0){
      return this.root;
    }
    return this.prevTrees[treeNum][this.depth][0];
  }

  static hashLeftRight(left, right) {    
    return utils.poseidon([left, right]);
  }

  static getZeroValue() {
    return utils.keccak256(Buffer.from('ZkDvp', 'utf8'));
  }

  static getZeroValueLevels(depth) {
    // Initialize empty array for levels
    const levels = [];

    // First level should be the leaf zero value
    levels.push(MerkleTree.getZeroValue());

    // Loop through remaining levels to root
    for (let level = 1; level < depth; level += 1) {
      // Push left right hash of level below's zero level
      levels.push(MerkleTree.hashLeftRight(levels[level - 1], levels[level - 1]));
    }
    return levels;
  }
}

module.exports = MerkleTree;
