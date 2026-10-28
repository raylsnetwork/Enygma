#!/usr/bin/env python3
"""PROTOTYPE (#131) — throwaway. Builds recording.js for the hop detail
prototype from #124's recording (recording_124.js, a real default-mode run).

It adds what the engine will emit once #129 lands, written by hand here:

- the Hop events the engine emits today (engine/*.go s.hop call sites),
  derived from the recorded log lines;
- a hop detail (`hopDetail`) on the hops that change protocol state, shaped
  as in #129: commitment, merklePath, publicInputs, settleChecks, leavesIn;
- a tree snapshot of both vaults at run start (proto field `trees`).

The Merkle trees are modelled for real (depth 8, numbered trees, a new tree
when a transaction's batch would fill the current one), but hashed with
sha256 mod the BN254 field instead of Poseidon, and every commitment,
salt and sibling is made up. Recorded short hex prefixes (commitments,
nullifiers) are kept and padded, so the values match the engine log.

    python3 make_recording.py > recording.js
"""
import hashlib
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).parent
DEPTH, LEAVES = 8, 256
P = 21888242871839275222246405745257275088548364400416034343698204186575808495617


def H(*parts):
    d = hashlib.sha256('|'.join(map(str, parts)).encode()).digest()
    return '0x%064x' % (int.from_bytes(d, 'big') % P)


def padded(prefix, seed):
    """A full-length value starting with a recorded short hex prefix."""
    p = prefix.rstrip('…').removeprefix('0x')
    return '0x' + p + H(seed)[2 + len(p):]


ZEROS = [H('zero leaf')]
for _ in range(DEPTH):
    ZEROS.append(H(ZEROS[-1], ZEROS[-1]))


class Tree:
    def __init__(self, num):
        self.num, self.leaves = num, []

    def level(self, nodes, l):
        return [H(nodes[i], nodes[i + 1] if i + 1 < len(nodes) else ZEROS[l]) for i in range(0, len(nodes), 2)]

    def root(self):
        nodes = [l['c'] for l in self.leaves]
        for l in range(DEPTH):
            nodes = self.level(nodes, l) if nodes else []
        return nodes[0] if nodes else ZEROS[DEPTH]

    def siblings(self, i):
        nodes, out = [l['c'] for l in self.leaves], []
        for l in range(DEPTH):
            j = i ^ 1
            out.append(nodes[j] if j < len(nodes) else ZEROS[l])
            nodes, i = self.level(nodes, l), i // 2
        return out


class Vault:
    def __init__(self, node, label, token):
        self.node, self.label, self.token = node, label, token
        self.tree, self.full, self.old, self.roots = Tree(0), [], {}, set()

    def insert(self, batch):
        """Insert one transaction's commitments; returns (tree, indices, root before)."""
        if len(self.tree.leaves) + len(batch) >= LEAVES:
            self.full.append({'tree': self.tree.num, 'leafCount': len(self.tree.leaves), 'root': self.tree.root()})
            self.old[self.tree.num] = self.tree
            self.tree = Tree(self.tree.num + 1)
        before = self.tree.root()
        start = len(self.tree.leaves)
        self.tree.leaves += batch
        self.roots.add((self.tree.num, self.tree.root()))
        return self.tree.num, list(range(start, start + len(batch))), before

    def snapshot(self):
        return {'vault': self.node, 'label': self.label, 'tree': self.tree.num, 'leafCount': len(self.tree.leaves),
                'leaves': [{'commitment': l['c'], 'owner': l['owner']} for l in self.tree.leaves],
                'root': self.tree.root(), 'fullTrees': list(self.full)}


VAULTS = {'erc': Vault('Erc20CoinVault', 'RaylsERC20 vault', 'RaylsERC20'), 'usdr': Vault('UsdrCoinVault', 'USDr vault', 'USDr')}
# Before this run: the RaylsERC20 vault has 25 leaves in tree 0 (the run's
# first rebuild reads 26 Commitment events, the shield's included). The USDr
# vault is made up near the end of tree 1, so the run rolls it into tree 2.
for i in range(25):
    VAULTS['erc'].insert([{'c': H('earlier erc', i), 'owner': None}])
for i in range(255 + 240):
    VAULTS['usdr'].insert([{'c': H('earlier usdr', i), 'owner': None}])
assert VAULTS['usdr'].tree.num == 1 and len(VAULTS['usdr'].tree.leaves) == 240
SNAPSHOT = [VAULTS['erc'].snapshot(), VAULTS['usdr'].snapshot()]

NAMES = ['Issuer', 'Sender', 'Ana', 'Bruno', 'Carla', 'Diego', 'Elena', 'Felipe']
node = lambda name: 'actor%d' % NAMES.index(name)
PAYOUTS = dict(zip(NAMES[2:], [120, 90, 150, 60, 200, 80]))
notes = {n: [] for n in NAMES}  # each actor's unspent RaylsERC20 notes
usdr = {}  # each actor's one USDr note

OPENING = ['pk_spend (owner)', 'salt', 'amount', 'token']
PRIVATE = ['sk_spend', 'amounts in and out', 'salts', 'Merkle path']


def new_note(vault, owner, amount, seed, commitment=None):
    return {'c': commitment or H('note', seed), 'owner': owner, 'amount': amount, 'vault': vault}


def land(vkey, batch):
    v = VAULTS[vkey]
    tree, idx, before = v.insert(batch)
    for n, i in zip(batch, idx):
        n.update(tree=tree, leaf=i)
    return {'kind': 'leavesIn', 'vault': v.node, 'tree': tree,
            'leaves': [{'index': i, 'commitment': n['c'], 'owner': n['owner']} for n, i in zip(batch, idx)],
            'rootBefore': before, 'rootAfter': v.tree.root(), 'treeStarted': idx[0] == 0}


def commitment_detail(vkey, n):
    return {'kind': 'commitment', 'vault': VAULTS[vkey].node, 'tree': n['tree'], 'leaf': n['leaf'],
            'commitment': n['c'], 'opening': OPENING, 'owner': n['owner'],
            # Not in #129's shape: the root after the deposit, so the tree view's root stays current.
            'rootAfter': VAULTS[vkey].tree.root()}


def path_of(vkey, n):
    """The note's Merkle path in its own tree, which may be a full one."""
    v = VAULTS[vkey]
    t = v.tree if v.tree.num == n['tree'] else v.old[n['tree']]
    return {'vault': v.node, 'tree': n['tree'], 'leaf': n['leaf'], 'commitment': n['c'],
            'siblings': t.siblings(n['leaf']), 'root': t.root(), 'owner': n['owner']}


src = (HERE / 'recording_124.js').read_text()
rec = json.loads(src[src.index('{'):src.rindex('}') + 1])

out = []
ctx = {}


def hop(t, step, frm, to, label, detail, hop_detail=None):
    e = {'kind': 'hop', 'step': step, 'from': frm, 'to': to, 'label': label, 'detail': detail, 'actor': 0, 't': t}
    if hop_detail:
        e['hopDetail'] = hop_detail
    out.append(e)


def derive(e):
    t, step, m = e['t'], e.get('step', ''), e.get('msg', '')
    if e['kind'] == 'step_started':
        ctx.clear()
        lab = e['label']
        if (r := re.match(r'^Pay (\w+)$', lab)):
            ctx.update(payer='Sender', payee=r[1], amount=PAYOUTS[r[1]])
        elif (r := re.match(r'^(\w+) → (\w+)$', lab)):
            ctx.update(payer=r[1], payee=r[2], amount=int(re.search(r'pays \w+ (\d+)', m)[1]))
        return
    if e['kind'] != 'log':
        return
    if (r := re.match(r'^(\w+)\s+pk_spend (\S+)\s+ML-KEM ek', m)):
        n = node(r[1])
        hop(t, step, n, n, 'spend key', 'Poseidon key pair; pk_spend goes into every note paid to ' + r[1])
        hop(t, step, n, n, 'view key', 'ML-KEM-768 key pair; 1184-byte pk_view, senders encrypt notes to it')
    elif (r := re.match(r'^funded (\d+) new accounts with ([\d.]+ ETH)', m)):
        for name in NAMES[1:1 + int(r[1])]:
            hop(t, step, 'funder', node(name), 'fund gas', f'{r[2]} to {name}\'s fresh signing account')
    elif (r := re.match(r'^UserRegistry\.register\((\w+)\).*\((\d+) \+ (\d+) bytes\)', m)):
        n = node(r[1])
        hop(t, step, n, n, 'audit ciphertexts', f'{r[2]}-byte ML-KEM capsule to the auditor + {r[3]}-byte AES-GCM encryption of the view key\'s seed')
        hop(t, step, n, 'UserRegistry', 'register', 'mined: most of the gas stores the 1184-byte view key on chain')
    elif (r := re.match(r"^auditor opened (\w+)'s audit ciphertexts", m)):
        hop(t, step, 'UserRegistry', 'auditor', 'recover view key', 'opens ' + r[1] + "'s audit ciphertexts")
    elif (r := re.search(r'openChannel\((\w+) → (\w+)\) via relayer → channel #(\d+)\s+block (\d+)\s+gas ([\d,]+)', m)):
        hop(t, step, node(r[1]), 'relayer', 'open channel', f"a shared secret encapsulated to {r[2]}'s view key")
        hop(t, step, 'relayer', 'TagChannelRegistry', 'openChannel', f'channel #{r[3]}, block {r[4]}, gas {r[5]}; the relayer is its on-chain sender')
    elif (r := re.match(r'^(\w+) checked (\d+) channel records, opened (\d+)', m)):
        hop(t, step, 'TagChannelRegistry', node(r[1]), 'scan channels', f"{r[2]} records checked, {r[3]} opened with {r[1]}'s view key")
    elif (r := re.match(r"^UsdrCoinVault\.depositV2\((\d+)\) → (\w+)'s note (\S+)\s+block (\d+)", m)):
        n = new_note('usdr', r[2], int(r[1]), 'usdr grant ' + r[2], padded(r[3], 'usdr grant ' + r[2]))
        land('usdr', [n])
        usdr[r[2]] = n
        hop(t, step, 'actor0', 'UsdrCoinVault', 'grant USDr', f"a {r[1]} USDr note for {r[2]}, encrypted to their view key, block {r[4]}",
            [commitment_detail('usdr', n)])
    elif re.match(r'^sender and \d+ participants each decrypted', m):
        for name in NAMES[1:]:
            hop(t, step, 'UsdrCoinVault', node(name), 'scan', f'{name} tried their view key on the vault\'s notes and opened their grant')
    elif (r := re.match(r'^Erc20CoinVault\.depositV2\((\d+)\) → block (\d+)\s+gas ([\d,]+)', m)):
        ctx['shield'] = (int(r[1]), r[2], r[3])
    elif (r := re.match(r"^sender's note commitment (\S+)", m)):
        amount, block, gas = ctx['shield']
        n = new_note('erc', 'Sender', amount, 'shield', padded(r[1], 'shield'))
        land('erc', [n])
        notes['Sender'].append(n)
        hop(t, step, 'actor1', 'Erc20CoinVault', 'shield', f'{amount} public RaylsERC20 (just minted) into one private note, block {block}, gas {gas}',
            [commitment_detail('erc', n)])
    elif (r := re.match(r"^(Payment2in|Payment) proof in (\d+) ms — (?:spends|merges) (\w+)'s ([\d +]+) notes?, nullifiers? (.+)$", m)):
        circuit, payer = r[1], r[3]
        amounts = [int(a) for a in r[4].split(' + ')]
        ins = []
        for a in amounts:
            ins.append(next(n for n in notes[payer] if n['amount'] == a and n not in ins))
        nulls = [padded(x.strip(), f'nullifier {ctx.get("payee")} {i} {x}') for i, x in enumerate(r[5].split(','))]
        paths = [path_of('erc', n) for n in ins]
        change = sum(amounts) - ctx['amount']
        payee = new_note('erc', ctx['payee'], ctx['amount'], f'pay {step} payee')
        chg = new_note('erc', payer, change, f'pay {step} change')
        ctx.update(circuit=circuit, ins=ins, nulls=nulls, paths=paths, out=[payee, chg])
        vid = '0x' + 'a34b0bb5f2d8c1674eb7cf53e06b813a31c5509c'.rjust(64, '0')
        if circuit == 'Payment':
            pub = [('message', H('msg', step)), ('treeNumbers[0]', paths[0]['tree']), ('merkleRoots[0]', paths[0]['root']),
                   ('nullifiers[0]', nulls[0]), ('commitmentsOut[0] · payee', payee['c']), ('commitmentsOut[1] · change', chg['c']),
                   ('contractAddress · vault', vid)]
        else:
            pub = [('message', H('msg', step)), ('treeNumbers[0]', paths[0]['tree']), ('treeNumbers[1]', paths[1]['tree']),
                   ('merkleRoots[0]', paths[0]['root']), ('merkleRoots[1]', paths[1]['root']),
                   ('nullifiers[0]', nulls[0]), ('nullifiers[1]', nulls[1]),
                   ('commitmentsOut[0] · payee', payee['c']), ('commitmentsOut[1] · change', chg['c']), ('contractAddress · vault', vid)]
        hop(t, step, node(payer), 'gnark', 'prove ' + circuit, f"{r[2]} ms; spends {payer}'s {r[4]} note(s)",
            [{'kind': 'merklePath', 'paths': paths},
             {'kind': 'publicInputs', 'circuit': circuit, 'public': [{'name': k, 'value': v} for k, v in pub], 'private': PRIVATE}])
    elif (r := re.match(r'^UsdrFee proof in (\d+) ms — (\d+) USDr fee note to the relayer, (\d+) USDr change back to (\w+)', m)):
        payer = r[4]
        n = usdr[payer]
        p = path_of('usdr', n)
        fee = new_note('usdr', 'Relayer', int(r[2]), f'fee {step}')
        chg = new_note('usdr', payer, int(r[3]), f'usdr change {step}')
        null = H('usdr nullifier', step)
        ctx.update(fee=dict(note=n, path=p, null=null, out=[fee, chg]))
        uid = '0x' + '20c856d1d33a80f574ffd67d411dbc7ce1c1f1b9'.rjust(64, '0')
        pub = [('message', H('usdr msg', step)), ('treeNumbers[0]', p['tree']), ('merkleRoots[0]', p['root']), ('nullifiers[0]', null),
               ('commitmentsOut[0] · fee to relayer', fee['c']), ('commitmentsOut[1] · USDr change', chg['c']),
               ('contractAddress · vault', uid), ('fee', r[2]), ('tokenId', '0')]
        hop(t, step, node(payer), 'gnark', 'prove UsdrFee', f'{r[1]} ms; {r[2]} USDr fee note to the relayer',
            [{'kind': 'merklePath', 'paths': [p]},
             {'kind': 'publicInputs', 'circuit': 'UsdrFee', 'public': [{'name': k, 'value': v} for k, v in pub],
              'private': ['sk_spend', 'USDr amount in and change', 'salts', 'Merkle path']}])
    elif (r := re.search(r'EnygmaDvp\.(\w+)\(\) via relayer → block (\d+)\s+gas ([\d,]+)', m)):
        payer, method, f = ctx['payer'], r[1], ctx['fee']
        roots = [{'vault': p['vault'], 'tree': p['tree'], 'root': p['root']} for p in ctx['paths']] + \
                [{'vault': f['path']['vault'], 'tree': f['path']['tree'], 'root': f['path']['root']}]
        hop(t, step, node(payer), 'relayer', 'relay', f'the relayer, not {payer}, submits the transaction')
        hop(t, step, 'relayer', 'EnygmaDvp', method, f'settles the payment and the USDr fee in one transaction, block {r[2]}, gas {r[3]}',
            [{'kind': 'settleChecks', 'roots': roots,
              'nullifiers': [{'vault': 'Erc20CoinVault', 'nullifier': x} for x in ctx['nulls']] + [{'vault': 'UsdrCoinVault', 'nullifier': f['null']}],
              'proofs': [ctx['circuit'], 'UsdrFee']}])
        for n in ctx['ins']:
            notes[payer].remove(n)
        payee, chg = ctx['out']
        erc = land('erc', [payee, chg])
        notes[ctx['payee']].append(payee)
        notes[payer].append(chg)
        tx = '0x' + H('tx', step)[2:10]
        hop(t, step, 'EnygmaDvp', 'Erc20CoinVault', 'notes in', f"{ctx['payee']}'s payment and {payer}'s change, tx {tx}…", [erc])
        fee, uchg = f['out']
        ud = land('usdr', [fee, uchg])
        usdr[payer] = uchg
        hop(t, step, 'EnygmaDvp', 'UsdrCoinVault', 'fee note in', f"the relayer's 1 USDr fee note, same tx {tx}…", [ud])
    elif (r := re.search(r"TagRegistry\.publishTag\(\) via relayer → (\w+)'s tag in block (\d+)\s+gas ([\d,]+)", m)):
        hop(t, step, node(ctx['payer']), 'relayer', 'tag', f'a tag only {r[1]} can recognise, carrying the note\'s opening')
        hop(t, step, 'relayer', 'TagRegistry', 'publishTag', f'block {r[2]}, gas {r[3]}')
    elif (r := re.match(r'^(\w+) checked (\d+) tags, matched (\d+)', m)):
        hop(t, step, 'TagRegistry', node(r[1]), 'scan tags', f'{r[2]} tags checked, {r[3]} matched')
        hop(t, step, 'Erc20CoinVault', node(r[1]), 'open note', f"the tagged note's commitment confirmed in the vault; {r[1]} decrypts only it")


for e in rec['events']:
    out.append(e)
    if e['kind'] == 'proto' and e['field'] == 'steps':
        out.append({'kind': 'proto', 'field': 'trees', 'value': json.dumps(SNAPSHOT), 'actor': 0, 't': e['t']})
    derive(e)

# Every vault root a proof used must be one the vault has had.
for e in out:
    for d in e.get('hopDetail', []):
        for p in d.get('paths', []):
            v = next(v for v in VAULTS.values() if v.node == p['vault'])
            assert (p['tree'], p['root']) in v.roots, p

for svc, lines in rec['services'].items():
    out += [{'kind': 'term', 'service': svc, 'line': l['line'], 't': l['t']} for l in lines]
out.sort(key=lambda e: e['t'])  # stable: the engine's own order is kept within a ms

sys.stdout.write('// PROTOTYPE (#131) — generated by make_recording.py; do not edit.\nwindow.RECORDING = ')
json.dump({'note': rec['note'] + ' Hops and hop details added by make_recording.py (#131): hashes are sha256 stand-ins, values made up.',
           'events': out}, sys.stdout, ensure_ascii=False, separators=(',', ':'))
sys.stdout.write(';\n')
