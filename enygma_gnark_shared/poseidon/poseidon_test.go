package poseidon

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

// The state widths the tables support: 1 to 9 inputs.
const minT, maxT = 2, 10

// TestFieldConstant accepts 0 and p-1, and panics on p, on the t=6 M[0][3]
// entry as it read before it was fixed (above p), on a negative value and on
// text that is not a number.
func TestFieldConstant(t *testing.T) {
	pMinus1 := "0x" + new(big.Int).Sub(bn254p, big.NewInt(1)).Text(16)
	for _, s := range []string{"0x0", pMinus1} {
		if got := "0x" + fieldConstant(s).Text(16); got != s {
			t.Errorf("fieldConstant(%s) = %s", s, got)
		}
	}
	for _, c := range []struct{ name, s, panic string }{
		{"p", "0x" + bn254p.Text(16), "outside the BN254 scalar field"},
		{"old t=6 M[0][3]", "0xc399e9f67aa40707a203feefb0b958bbdad5cec5ca34901d253d026a2419f6a2", "outside the BN254 scalar field"},
		{"negative", "-0x1", "outside the BN254 scalar field"},
		{"not a number", "0x12g4", "invalid constant"},
	} {
		t.Run(c.name, func(t *testing.T) {
			msg := panicText(func() { fieldConstant(c.s) })
			if !strings.Contains(msg, c.panic) {
				t.Errorf("fieldConstant(%s) panicked with %q, want %q", c.s, msg, c.panic)
			}
		})
	}
}

// panicText runs f and returns what it panicked with, or "" if it returned.
func panicText(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	f()
	return ""
}

// TestTablesParse loads every table at every supported width. Each getter
// panics on an entry outside the field, which a typo in a table can make.
func TestTablesParse(t *testing.T) {
	getters := []struct {
		name string
		get  func(int)
	}{
		{"C", func(w int) { GetPoseidonC(w) }},
		{"S", func(w int) { GetPoseidonS(w) }},
		{"M", func(w int) { GetPoseidonM(w) }},
		{"P", func(w int) { GetPoseidonP(w) }},
	}
	for w := minT; w <= maxT; w++ {
		for _, g := range getters {
			t.Run(fmt.Sprintf("t=%d/%s", w, g.name), func(t *testing.T) {
				if msg := panicText(func() { g.get(w) }); msg != "" {
					t.Error(msg)
				}
			})
		}
	}
}

// TestVectorsCoverEveryWidth keeps, for each supported width, a vector of
// Poseidon (initial state 0, one output) and one of PoseidonEx (another
// initial state, all t outputs), so the tests below read every table.
func TestVectorsCoverEveryWidth(t *testing.T) {
	for w := minT; w <= maxT; w++ {
		var plain, ex bool
		for _, v := range vectors {
			if v.t() != w {
				continue
			}
			plain = plain || (v.init == "0" && len(v.outs) == 1)
			ex = ex || (v.init != "0" && len(v.outs) == w)
		}
		if !plain || !ex {
			t.Errorf("t=%d: Poseidon vector %v, PoseidonEx vector %v", w, plain, ex)
		}
	}
}

// TestNativePoseidonExMatchesCircomlib checks the native hash against every
// vector.
func TestNativePoseidonExMatchesCircomlib(t *testing.T) {
	for _, v := range vectors {
		t.Run(v.label(), func(t *testing.T) {
			init := integers(t, []string{v.init})[0]
			got := NativePoseidonEx(integers(t, v.inputs), init, len(v.outs))
			for i, want := range integers(t, v.outs) {
				if got[i].Cmp(want) != 0 {
					t.Errorf("output %d is %s, want %s", i, got[i], want)
				}
			}
		})
	}
}

// hashCircuit asserts Poseidon(In) = Out, the call the deployed circuits make.
type hashCircuit struct {
	In  []frontend.Variable
	Out frontend.Variable `gnark:",public"`
}

func (c *hashCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(Poseidon(api, c.In), c.Out)
	return nil
}

// hashExCircuit asserts PoseidonEx(In, Init, len(Out)) = Out.
type hashExCircuit struct {
	In   []frontend.Variable
	Init frontend.Variable
	Out  []frontend.Variable `gnark:",public"`
}

func (c *hashExCircuit) Define(api frontend.API) error {
	out := PoseidonEx(api, c.In, c.Init, len(c.Out))
	for i := range c.Out {
		api.AssertIsEqual(out[i], c.Out[i])
	}
	return nil
}

// gadgetCircuits returns, for v, the circuit to compile (Poseidon for a
// vector with initial state 0 and one output, else PoseidonEx), an
// assignment of v's values, and that assignment with its first output off
// by one.
func gadgetCircuits(t *testing.T, v vector) (shape, good, bad frontend.Circuit) {
	t.Helper()
	ints := integers(t, v.inputs)
	in := make([]frontend.Variable, len(ints))
	for i, x := range ints {
		in[i] = x
	}
	outs := integers(t, v.outs)
	wrong := new(big.Int).Add(outs[0], big.NewInt(1))
	wrong.Mod(wrong, bn254p)
	if v.init == "0" && len(outs) == 1 {
		return &hashCircuit{In: make([]frontend.Variable, len(in))},
			&hashCircuit{In: in, Out: outs[0]},
			&hashCircuit{In: in, Out: wrong}
	}
	init := integers(t, []string{v.init})[0]
	goodOut := make([]frontend.Variable, len(outs))
	badOut := make([]frontend.Variable, len(outs))
	for i, o := range outs {
		goodOut[i], badOut[i] = o, o
	}
	badOut[0] = wrong
	return &hashExCircuit{In: make([]frontend.Variable, len(in)), Out: make([]frontend.Variable, len(outs))},
		&hashExCircuit{In: in, Init: init, Out: goodOut},
		&hashExCircuit{In: in, Init: init, Out: badOut}
}

// TestGadgetMatchesCircomlib checks the in-circuit hash against every
// vector on gnark's test engine: the vector's outputs solve the circuit,
// and an output off by one does not.
func TestGadgetMatchesCircomlib(t *testing.T) {
	for _, v := range vectors {
		t.Run(v.label(), func(t *testing.T) {
			shape, good, bad := gadgetCircuits(t, v)
			if err := test.IsSolved(shape, good, ecc.BN254.ScalarField()); err != nil {
				t.Errorf("the vector's outputs do not solve the circuit: %v", err)
			}
			if test.IsSolved(shape, bad, ecc.BN254.ScalarField()) == nil {
				t.Error("an output off by one solves the circuit")
			}
		})
	}
}

// TestCompiledGadgetMatchesCircomlib checks the same on the compiled R1CS,
// the form keygen and the provers use.
func TestCompiledGadgetMatchesCircomlib(t *testing.T) {
	field := ecc.BN254.ScalarField()
	for _, v := range vectors {
		t.Run(v.label(), func(t *testing.T) {
			shape, good, bad := gadgetCircuits(t, v)
			ccs, err := frontend.Compile(field, r1cs.NewBuilder, shape)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			goodW, err := frontend.NewWitness(good, field)
			if err != nil {
				t.Fatalf("witness: %v", err)
			}
			if err := ccs.IsSolved(goodW); err != nil {
				t.Errorf("the vector's outputs do not solve the R1CS: %v", err)
			}
			badW, err := frontend.NewWitness(bad, field)
			if err != nil {
				t.Fatalf("witness: %v", err)
			}
			if ccs.IsSolved(badW) == nil {
				t.Error("an output off by one solves the R1CS")
			}
		})
	}
}

// integers parses decimal strings.
func integers(t *testing.T, ss []string) []*big.Int {
	t.Helper()
	out := make([]*big.Int, len(ss))
	for i, s := range ss {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("vector value %q is not a decimal integer", s)
		}
		out[i] = v
	}
	return out
}

// vector is a Poseidon test vector, in decimal: PoseidonEx(inputs, init,
// len(outs)) is outs, at width t = len(inputs) + 1. circomlibjs 0.1.7
// computed each one, with both its optimised and its reference
// implementation, and go-iden3-crypto v0.0.17 gives the same values. The
// "published" ones are from circomlibjs's and iden3's own tests, which also
// publish the "inputs 1..n" vectors of t = 2, 3 and 7.
type vector struct {
	name   string
	inputs []string
	init   string
	outs   []string
}

func (v vector) t() int { return len(v.inputs) + 1 }

func (v vector) label() string { return fmt.Sprintf("t=%d/%s", v.t(), v.name) }

// largest returns the n largest field elements, p-1 down to p-n.
func largest(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = new(big.Int).Sub(bn254p, big.NewInt(int64(i+1))).String()
	}
	return out
}

var vectors = []vector{
	// t = 2
	{name: "inputs 1..n", inputs: []string{"1"}, init: "0", outs: []string{"18586133768512220936620570745912940619677854269274689475585506675881198879027"}},
	{name: "inputs p-1..p-n", inputs: largest(1), init: "0", outs: []string{"3366645945435192953002076803303112651887535928162668198103357554665518664470"}},
	{name: "state and all outputs", inputs: []string{"1"}, init: "2007", outs: []string{
		"8955978660397222425999143310975909412258184920391926469816585630057837279299",
		"12213186159011003815629784271946931494808282229076635829009512765827336634527",
	}},
	// t = 3
	{name: "inputs 1..n", inputs: []string{"1", "2"}, init: "0", outs: []string{"7853200120776062878684798364095072458815029376092732009249414926327459813530"}},
	{name: "inputs p-1..p-n", inputs: largest(2), init: "0", outs: []string{"6149179154952470170078400310622041627793456726695327690341847329306121599910"}},
	{name: "state and all outputs", inputs: []string{"1", "2"}, init: "3007", outs: []string{
		"12752801005557647707307664070626744457445276377532148344712375313682003346555",
		"20913581300209169154665087023817644786539253491997274056872605348947087987140",
		"20970890085932341213325536698394001178168304761087955838404614923310376240375",
	}},
	// t = 4
	{name: "inputs 1..n", inputs: []string{"1", "2", "3"}, init: "0", outs: []string{"6542985608222806190361240322586112750744169038454362455181422643027100751666"}},
	{name: "inputs p-1..p-n", inputs: largest(3), init: "0", outs: []string{"20133197287301041945818928843432871492701166386418821647132231623255907241873"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3"}, init: "4007", outs: []string{
		"6112910731692281037383586965235205044617902977819302789027845254378359203872",
		"8025137698579881153436848691099884233943352359062621193723645309439299503511",
		"5433624483475024039039905320221269706556030573790247943211067576749236143006",
		"20870645639589806064644918689826716283215803481443740597549093230346280784580",
	}},
	// t = 5
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4"}, init: "0", outs: []string{"18821383157269793795438455681495246036402687001665670618754263018637548127333"}},
	{name: "inputs p-1..p-n", inputs: largest(4), init: "0", outs: []string{"1600226338661258867773135178948901161771826415426494760737987633288080075054"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4"}, init: "5007", outs: []string{
		"20830308226446198694853117152000092413163592881886526308774700129050296802343",
		"12517376589606804133824744000768263486020036399873314471797705471310983641993",
		"5550756023408834850440767736928160023218175739116431219751203467078160016567",
		"13553504147835939226730935595903759200089666009555487633521113098866616892355",
		"12954070294263523025426269581076588259425176464893058633683201906311341490490",
	}},
	// t = 6
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4", "5"}, init: "0", outs: []string{"6183221330272524995739186171720101788151706631170188140075976616310159254464"}},
	{name: "inputs p-1..p-n", inputs: largest(5), init: "0", outs: []string{"21114110134654027945979568917342954732994702179875160006836402013753920548839"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4", "5"}, init: "6007", outs: []string{
		"8349159285900507445171063187080844889593017576436405807965466417687043963567",
		"7730406798125869874242651910540493364238497575959759159890813685375202664577",
		"11754791645831817396856583669489237336454248922810931353869729829549956155073",
		"1928273249041176969057422956154982696694578930346255764778760823603687697960",
		"21145991189106574509410450327691511348525037278138581210236646773877562280601",
		"546993504630495030398913216350184573388174211624141090943233198519268695018",
	}},
	{name: "published 1,2,0,0,0", inputs: []string{"1", "2", "0", "0", "0"}, init: "0", outs: []string{"1018317224307729531995786483840663576608797660851238720571059489595066344487"}},
	{name: "published 3,4,0,0,0", inputs: []string{"3", "4", "0", "0", "0"}, init: "0", outs: []string{"5811595552068139067952687508729883632420015185677766880877743348592482390548"}},
	// t = 7
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4", "5", "6"}, init: "0", outs: []string{"20400040500897583745843009878988256314335038853985262692600694741116813247201"}},
	{name: "inputs p-1..p-n", inputs: largest(6), init: "0", outs: []string{"16325342260494017668076510966261805737279452815711533758687116124985373877350"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4", "5", "6"}, init: "7007", outs: []string{
		"10849540636325934354385187642887422875599495013643831840918953606341009151200",
		"8272990467508976601748823309951982670077385379984681886911725060798380933053",
		"17520850486669396625161775165354132735376501958631013956045603427003246547946",
		"6206262728431546535267261707886091825840925903959319648791242593860406387564",
		"6425568219436015212252233006175765992825082085481644848695332011658995221626",
		"13842794337775045334274791603799774170367320902450434504999242161198612399811",
		"5119055744713926633020736549750723883932290941212762946215031996297339791178",
	}},
	{name: "published 1,2,0,0,0,0", inputs: []string{"1", "2", "0", "0", "0", "0"}, init: "0", outs: []string{"15336558801450556532856248569924170992202208561737609669134139141992924267169"}},
	{name: "published 3,4,0,0,0,0", inputs: []string{"3", "4", "0", "0", "0", "0"}, init: "0", outs: []string{"12263118664590987767234828103155242843640892839966517009184493198782366909018"}},
	// t = 8
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4", "5", "6", "7"}, init: "0", outs: []string{"12748163991115452309045839028154629052133952896122405799815156419278439301912"}},
	{name: "inputs p-1..p-n", inputs: largest(7), init: "0", outs: []string{"18680167046451237191589624047637541978150381432388869411011252074539198881113"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4", "5", "6", "7"}, init: "8007", outs: []string{
		"7911059994463530589782915354906396388772773654903641147051517887896458509485",
		"15476124918735473870177266722591065281380233508739685325230892580448009805924",
		"16119521576009757874488342217873033939956478385271736504498669620935961512122",
		"10374393838473231896922527907059208388636712135623924292484463888707359892313",
		"14951777208778630627680912515389505820712490457554487520568234186369375743392",
		"8383138724873328838546502162613065969754709790068765917726799332103763357982",
		"6272190301541418130936727482940583253176214884305082982866421215610964535135",
		"19632216133201583508941750967191390779773658316544605069462486114672383717970",
	}},
	// t = 9
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4", "5", "6", "7", "8"}, init: "0", outs: []string{"18604317144381847857886385684060986177838410221561136253933256952257712543953"}},
	{name: "inputs p-1..p-n", inputs: largest(8), init: "0", outs: []string{"9342052913489841743058620073984187375962719467586455256350481489535884266796"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4", "5", "6", "7", "8"}, init: "9007", outs: []string{
		"20852817571663601166281618961662117647904745422959397970522366963019438141806",
		"16305757041814682952290950033776441574195889962945744330978178141308504602215",
		"10653997317800829706668559215741158820396318338821443331900692892130732566869",
		"5199191429109655298940134481636164962032503920043970913441013717431541710478",
		"17491726932794197144846334064106402372456330327108382747515085984049134380973",
		"16166571793019452971978705553788937490343903809349455567206426205864737255979",
		"7436068795175366901463255486745131601364738940015397643181328830845320380349",
		"15612163557394914809382424262328296794164358949248366620110028863300270072738",
		"18123864962816836615886777108243937010405408096366119340365543759585383427378",
	}},
	// t = 10
	{name: "inputs 1..n", inputs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, init: "0", outs: []string{"13589767895268936107593642967621470491511464502761040466226072462545218539640"}},
	{name: "inputs p-1..p-n", inputs: largest(9), init: "0", outs: []string{"21377787756945379155660059727199270645557311312190881589137574386798465299698"}},
	{name: "state and all outputs", inputs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, init: "10007", outs: []string{
		"11534623816853854255110086883425996350027099104730450222376102158889203047865",
		"19906965224551478170490255499855638208581448371535906984212206101351220773241",
		"3129193124718787909259021436681798908853634782966274060643526084670546951044",
		"18341058895678255723246955188040298670751840293954776885329219757183085754696",
		"11548758667390890359430510511325243011236562512225854388765403930325691544149",
		"7062921039975115770648040694474509205818333353060620307336921234607278905687",
		"10891134999854515579440066001187821752650011017273595991675409064572855854504",
		"4244426129140372083776453123462752875444520890260093356512847203702534985226",
		"4163054257437267855396810909072785541963706868435394569924629695678908551646",
		"3447117125932665845907475945262977268774324500460726812840229633428729666288",
	}},
}
