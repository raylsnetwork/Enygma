// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

/// @title Groth16 verifier template.
/// @author Remco Bloemen
/// @notice Supports verifying Groth16 proofs. Proofs can be in uncompressed
/// (256 bytes) and compressed (128 bytes) format. A view function is provided
/// to compress proofs.
/// @notice See <https://2π.com/23/bn254-compression> for further explanation.
contract Verifier {
    /// Some of the provided public input values are larger than the field modulus.
    /// @dev Public input elements are not automatically reduced, as this is can be
    /// a dangerous source of bugs.
    error PublicInputNotInField();

    /// The proof is invalid.
    /// @dev This can mean that provided Groth16 proof points are not on their
    /// curves, that pairing equation fails, or that the proof is not for the
    /// provided public input.
    error ProofInvalid();

    // Addresses of precompiles
    uint256 constant PRECOMPILE_MODEXP = 0x05;
    uint256 constant PRECOMPILE_ADD = 0x06;
    uint256 constant PRECOMPILE_MUL = 0x07;
    uint256 constant PRECOMPILE_VERIFY = 0x08;

    // Base field Fp order P and scalar field Fr order R.
    // For BN254 these are computed as follows:
    //     t = 4965661367192848881
    //     P = 36⋅t⁴ + 36⋅t³ + 24⋅t² + 6⋅t + 1
    //     R = 36⋅t⁴ + 36⋅t³ + 18⋅t² + 6⋅t + 1
    uint256 constant P =
        0x30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47;
    uint256 constant R =
        0x30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001;

    // Extension field Fp2 = Fp[i] / (i² + 1)
    // Note: This is the complex extension field of Fp with i² = -1.
    //       Values in Fp2 are represented as a pair of Fp elements (a₀, a₁) as a₀ + a₁⋅i.
    // Note: The order of Fp2 elements is *opposite* that of the pairing contract, which
    //       expects Fp2 elements in order (a₁, a₀). This is also the order in which
    //       Fp2 elements are encoded in the public interface as this became convention.

    // Constants in Fp
    uint256 constant FRACTION_1_2_FP =
        0x183227397098d014dc2822db40c0ac2ecbc0b548b438e5469e10460b6c3e7ea4;
    uint256 constant FRACTION_27_82_FP =
        0x2b149d40ceb8aaae81be18991be06ac3b5b4c5e559dbefa33267e6dc24a138e5;
    uint256 constant FRACTION_3_82_FP =
        0x2fcd3ac2a640a154eb23960892a85a68f031ca0c8344b23a577dcf1052b9e775;

    // Exponents for inversions and square roots mod P
    uint256 constant EXP_INVERSE_FP =
        0x30644E72E131A029B85045B68181585D97816A916871CA8D3C208C16D87CFD45; // P - 2
    uint256 constant EXP_SQRT_FP =
        0xC19139CB84C680A6E14116DA060561765E05AA45A1C72A34F082305B61F3F52; // (P + 1) / 4;

    // Groth16 alpha point in G1
    uint256 constant ALPHA_X =
        13452987024514493238302764768532550799791872341681751784379252289339684448809;
    uint256 constant ALPHA_Y =
        20263012823168336674104885928861930077174947866040292904761898787783883417153;

    // Groth16 beta point in G2 in powers of i
    uint256 constant BETA_NEG_X_0 =
        1267026722123227386108156732184031155355570530013929850080124423649559314552;
    uint256 constant BETA_NEG_X_1 =
        4429056995768446995830003573068992552131195104127412821554664185688676349924;
    uint256 constant BETA_NEG_Y_0 =
        9909380094306090274100352359346856816165371889930780974047502402349063989742;
    uint256 constant BETA_NEG_Y_1 =
        6431319429850906259078669659902777050865733892327096512746442292391324911196;

    // Groth16 gamma point in G2 in powers of i
    uint256 constant GAMMA_NEG_X_0 =
        14207884102035270156859660306291681274703017514825089356082810221417425369011;
    uint256 constant GAMMA_NEG_X_1 =
        19286711229289635030253898027016866702112843342737010581506074640956921393358;
    uint256 constant GAMMA_NEG_Y_0 =
        11055739339848595550367659316733713448092577123170863250795665813894611786276;
    uint256 constant GAMMA_NEG_Y_1 =
        2405726906966028382957393377249070701259180086710803062607714843163695374977;

    // Groth16 delta point in G2 in powers of i
    uint256 constant DELTA_NEG_X_0 =
        6416078293415645204682396566040524103051693700835713612681951787844413383789;
    uint256 constant DELTA_NEG_X_1 =
        19959244090637711140201438865639699761522043230577827486422712439332865263568;
    uint256 constant DELTA_NEG_Y_0 =
        20672530920303307786280187458767286478268570574180140136226120598455128939490;
    uint256 constant DELTA_NEG_Y_1 =
        1735157527891095268459604815163358254044883856979210504548026651565511412714;

    // Constant and public input points
    uint256 constant CONSTANT_X =
        11074832153051311114895465655951371724632842781121105550480083370822722571667;
    uint256 constant CONSTANT_Y =
        16721076930585491904206793864305064556210341654433282176983308711160602226566;
    uint256 constant PUB_0_X =
        20118234485830399446826208896649515171974074409855418519499905423695310413819;
    uint256 constant PUB_0_Y =
        18381065296940116660578278039459018580067193372753251689244236768748071871373;
    uint256 constant PUB_1_X =
        18030810249144724834628181337197853372127919190767211867019154700782625551597;
    uint256 constant PUB_1_Y =
        20064987642291529612753410309527551043223489905246692801768092219360520247334;
    uint256 constant PUB_2_X =
        21116607414852214421000757766896110922060128907602525846743513359425856147348;
    uint256 constant PUB_2_Y =
        13223003910828704318281037688085329461485749853867901908911868044759169155577;
    uint256 constant PUB_3_X =
        12444574843388820611914934486728518491513860020987483071406612242459124299187;
    uint256 constant PUB_3_Y =
        16423828144982175385890392687387000691262022648086986598146429091687662027517;
    uint256 constant PUB_4_X =
        9382755179751530794330851293284093418230121622336435233709569647717529232584;
    uint256 constant PUB_4_Y =
        15166548251926423189349842647398110240468829939798914568386759730524936322593;
    uint256 constant PUB_5_X =
        17980572895132089742085714819206781819423351503934858015688034641660493345674;
    uint256 constant PUB_5_Y =
        6728954611480908583422025979016702966714637967597407673652921322979634189106;
    uint256 constant PUB_6_X =
        17034094604998367661258103667215271746684467603897729249114537269273281130675;
    uint256 constant PUB_6_Y =
        18903582563509679567809968315214292319975338274184208603392992368114563207518;
    uint256 constant PUB_7_X =
        14168684351423301462914811241693804422739728419496051745919525508877383154179;
    uint256 constant PUB_7_Y =
        15783757112186645161191027685217099872261396879783250106479541352561629299276;
    uint256 constant PUB_8_X =
        19071136166960056117119388088250542029933534438080492617085995591082140831421;
    uint256 constant PUB_8_Y =
        8460138985330875131191880792435970159894209150109695711942354146849534004681;
    uint256 constant PUB_9_X =
        20737552311016346446704337420575209375267615939221427037338648993387090453652;
    uint256 constant PUB_9_Y =
        3734354982061471753445617408506628431903852888117195480532161242972496738177;
    uint256 constant PUB_10_X =
        20939157357218585193074501494681137123977573577550038402401879731718786403455;
    uint256 constant PUB_10_Y =
        1778426105627641565423509831991356311642118368008037130852455594140065102909;
    uint256 constant PUB_11_X =
        8660132219523875813974835036162383279420352811843960003364565182276140483845;
    uint256 constant PUB_11_Y =
        1170178862642039107953639075881091101214723572399036190770655617964139767824;
    uint256 constant PUB_12_X =
        9778566409544057163849078824384414005630843072031916902704921406470201987062;
    uint256 constant PUB_12_Y =
        21496077542580913141204580565849765560448883835763323506328100686293245515109;
    uint256 constant PUB_13_X =
        9179739780177927915332297864594470273015175342679526662999481417472387022994;
    uint256 constant PUB_13_Y =
        11599366503994574086404873419749234023553470084562718564486291293167259940553;
    uint256 constant PUB_14_X =
        9409698199248450771331727017479640295179889883696021966673956152974143354862;
    uint256 constant PUB_14_Y =
        1603291371247844330998515917336107318001434318413931769649769425659496311675;
    uint256 constant PUB_15_X =
        11484526004491022641099429616456016268935934185909399960699580484769096366922;
    uint256 constant PUB_15_Y =
        20068730089649582144523916839800358939884694253164053998875783779451918073267;
    uint256 constant PUB_16_X =
        13250041983059681087404443482608005941524025154011524164356839762354158969673;
    uint256 constant PUB_16_Y =
        18045165829044943763322876736488192567610297667776748111361380004721560000607;
    uint256 constant PUB_17_X =
        1388975181279779103086310510532693159367597377288832827881685307331226901603;
    uint256 constant PUB_17_Y =
        13149149354225664204312313779424519753783970194378987458825759741129946663096;
    uint256 constant PUB_18_X =
        5055988601347720143841842293896615826274494484575140687130452176249073179318;
    uint256 constant PUB_18_Y =
        18513484802157416757616588990021888034392011831783975008219840168934426428649;
    uint256 constant PUB_19_X =
        18879168426319287288802368344122988436467734183432040218169241327732500745645;
    uint256 constant PUB_19_Y =
        13022384693882801364929790517725273440758712124792532421214724509377221176789;
    uint256 constant PUB_20_X =
        19976081894616051164889824344427054050824984517353607588607362973604484577394;
    uint256 constant PUB_20_Y =
        5983787584897990677942970266646512680194136841637991701809280731889605381484;
    uint256 constant PUB_21_X =
        15051042569451724588768177310220200179083451644741451466268202014589614301663;
    uint256 constant PUB_21_Y =
        3669308272169221097959209202376920100083996422798785685790578109493337504347;
    uint256 constant PUB_22_X =
        13504019156578644634603496037262889811979172466483990080752235268565249781405;
    uint256 constant PUB_22_Y =
        10266523443544558305293433583542990920751685138645033515438341800798576445900;
    uint256 constant PUB_23_X =
        12699939667444846171976994241399388884783744682413056928931848695773731152082;
    uint256 constant PUB_23_Y =
        12862261849153164306759729209190906548035905050341909608233308673158428356880;
    uint256 constant PUB_24_X =
        11713936564542097861120589250897395526842424607017063670576515690655429070292;
    uint256 constant PUB_24_Y =
        1956279734266096290989285855114041360925050371989429734670494442641908710052;
    uint256 constant PUB_25_X =
        11174267995327129867579743680643561831063532920566798672797617398818986270465;
    uint256 constant PUB_25_Y =
        11018579800178614660231613148375443239947889664082733086050165786824226761445;
    uint256 constant PUB_26_X =
        18966261487753626211581650736865874058933589329119717431377495789534835679025;
    uint256 constant PUB_26_Y =
        7077233556852161269421476356748402802632245910071665111462761495003377874182;
    uint256 constant PUB_27_X =
        114756142288480372895097702852963884955140497887857126897485889821423670774;
    uint256 constant PUB_27_Y =
        14630892251454528304938547765315644334503662477524779233192705008395363897493;
    uint256 constant PUB_28_X =
        10684037174327821412632170482380557944809893506296400455672144568773777137242;
    uint256 constant PUB_28_Y =
        4496025534686482261348951191607764678951343168049354544150192693263003329219;
    uint256 constant PUB_29_X =
        13569213775112355933691305157925296148986833064437067492437497191032917484165;
    uint256 constant PUB_29_Y =
        15557037326761279812596272852473282114412211239500098913972448942377865824474;
    uint256 constant PUB_30_X =
        6751039068806064525200573784502756881913455899438575061516216038896093073087;
    uint256 constant PUB_30_Y =
        10286873822828534514012202147046581070521876933815859928262673296068838402354;
    uint256 constant PUB_31_X =
        6638782333779646747087844237911929820267084805634512564169277703278670105160;
    uint256 constant PUB_31_Y =
        20255941900358409996188994433979939667827985602888535071588454582401626998803;

    /// Negation in Fp.
    /// @notice Returns a number x such that a + x = 0 in Fp.
    /// @notice The input does not need to be reduced.
    /// @param a the base
    /// @return x the result
    function negate(uint256 a) internal pure returns (uint256 x) {
        unchecked {
            x = (P - (a % P)) % P; // Modulo is cheaper than branching
        }
    }

    /// Exponentiation in Fp.
    /// @notice Returns a number x such that a ^ e = x in Fp.
    /// @notice The input does not need to be reduced.
    /// @param a the base
    /// @param e the exponent
    /// @return x the result
    function exp(uint256 a, uint256 e) internal view returns (uint256 x) {
        bool success;
        assembly ("memory-safe") {
            let f := mload(0x40)
            mstore(f, 0x20)
            mstore(add(f, 0x20), 0x20)
            mstore(add(f, 0x40), 0x20)
            mstore(add(f, 0x60), a)
            mstore(add(f, 0x80), e)
            mstore(add(f, 0xa0), P)
            success := staticcall(gas(), PRECOMPILE_MODEXP, f, 0xc0, f, 0x20)
            x := mload(f)
        }
        if (!success) {
            // Exponentiation failed.
            // Should not happen.
            revert ProofInvalid();
        }
    }

    /// Invertsion in Fp.
    /// @notice Returns a number x such that a * x = 1 in Fp.
    /// @notice The input does not need to be reduced.
    /// @notice Reverts with ProofInvalid() if the inverse does not exist
    /// @param a the input
    /// @return x the solution
    function invert_Fp(uint256 a) internal view returns (uint256 x) {
        x = exp(a, EXP_INVERSE_FP);
        if (mulmod(a, x, P) != 1) {
            // Inverse does not exist.
            // Can only happen during G2 point decompression.
            revert ProofInvalid();
        }
    }

    /// Square root in Fp.
    /// @notice Returns a number x such that x * x = a in Fp.
    /// @notice Will revert with InvalidProof() if the input is not a square
    /// or not reduced.
    /// @param a the square
    /// @return x the solution
    function sqrt_Fp(uint256 a) internal view returns (uint256 x) {
        x = exp(a, EXP_SQRT_FP);
        if (mulmod(x, x, P) != a) {
            // Square root does not exist or a is not reduced.
            // Happens when G1 point is not on curve.
            revert ProofInvalid();
        }
    }

    /// Square test in Fp.
    /// @notice Returns whether a number x exists such that x * x = a in Fp.
    /// @notice Will revert with InvalidProof() if the input is not a square
    /// or not reduced.
    /// @param a the square
    /// @return x the solution
    function isSquare_Fp(uint256 a) internal view returns (bool) {
        uint256 x = exp(a, EXP_SQRT_FP);
        return mulmod(x, x, P) == a;
    }

    /// Square root in Fp2.
    /// @notice Fp2 is the complex extension Fp[i]/(i^2 + 1). The input is
    /// a0 + a1 ⋅ i and the result is x0 + x1 ⋅ i.
    /// @notice Will revert with InvalidProof() if
    ///   * the input is not a square,
    ///   * the hint is incorrect, or
    ///   * the input coefficients are not reduced.
    /// @param a0 The real part of the input.
    /// @param a1 The imaginary part of the input.
    /// @param hint A hint which of two possible signs to pick in the equation.
    /// @return x0 The real part of the square root.
    /// @return x1 The imaginary part of the square root.
    function sqrt_Fp2(
        uint256 a0,
        uint256 a1,
        bool hint
    ) internal view returns (uint256 x0, uint256 x1) {
        // If this square root reverts there is no solution in Fp2.
        uint256 d = sqrt_Fp(addmod(mulmod(a0, a0, P), mulmod(a1, a1, P), P));
        if (hint) {
            d = negate(d);
        }
        // If this square root reverts there is no solution in Fp2.
        x0 = sqrt_Fp(mulmod(addmod(a0, d, P), FRACTION_1_2_FP, P));
        x1 = mulmod(a1, invert_Fp(mulmod(x0, 2, P)), P);

        // Check result to make sure we found a root.
        // Note: this also fails if a0 or a1 is not reduced.
        if (
            a0 != addmod(mulmod(x0, x0, P), negate(mulmod(x1, x1, P)), P) ||
            a1 != mulmod(2, mulmod(x0, x1, P), P)
        ) {
            revert ProofInvalid();
        }
    }

    /// Compress a G1 point.
    /// @notice Reverts with InvalidProof if the coordinates are not reduced
    /// or if the point is not on the curve.
    /// @notice The point at infinity is encoded as (0,0) and compressed to 0.
    /// @param x The X coordinate in Fp.
    /// @param y The Y coordinate in Fp.
    /// @return c The compresed point (x with one signal bit).
    function compress_g1(
        uint256 x,
        uint256 y
    ) internal view returns (uint256 c) {
        if (x >= P || y >= P) {
            // G1 point not in field.
            revert ProofInvalid();
        }
        if (x == 0 && y == 0) {
            // Point at infinity
            return 0;
        }

        // Note: sqrt_Fp reverts if there is no solution, i.e. the x coordinate is invalid.
        uint256 y_pos = sqrt_Fp(addmod(mulmod(mulmod(x, x, P), x, P), 3, P));
        if (y == y_pos) {
            return (x << 1) | 0;
        } else if (y == negate(y_pos)) {
            return (x << 1) | 1;
        } else {
            // G1 point not on curve.
            revert ProofInvalid();
        }
    }

    /// Decompress a G1 point.
    /// @notice Reverts with InvalidProof if the input does not represent a valid point.
    /// @notice The point at infinity is encoded as (0,0) and compressed to 0.
    /// @param c The compresed point (x with one signal bit).
    /// @return x The X coordinate in Fp.
    /// @return y The Y coordinate in Fp.
    function decompress_g1(
        uint256 c
    ) internal view returns (uint256 x, uint256 y) {
        // Note that X = 0 is not on the curve since 0³ + 3 = 3 is not a square.
        // so we can use it to represent the point at infinity.
        if (c == 0) {
            // Point at infinity as encoded in EIP196 and EIP197.
            return (0, 0);
        }
        bool negate_point = c & 1 == 1;
        x = c >> 1;
        if (x >= P) {
            // G1 x coordinate not in field.
            revert ProofInvalid();
        }

        // Note: (x³ + 3) is irreducible in Fp, so it can not be zero and therefore
        //       y can not be zero.
        // Note: sqrt_Fp reverts if there is no solution, i.e. the point is not on the curve.
        y = sqrt_Fp(addmod(mulmod(mulmod(x, x, P), x, P), 3, P));
        if (negate_point) {
            y = negate(y);
        }
    }

    /// Compress a G2 point.
    /// @notice Reverts with InvalidProof if the coefficients are not reduced
    /// or if the point is not on the curve.
    /// @notice The G2 curve is defined over the complex extension Fp[i]/(i^2 + 1)
    /// with coordinates (x0 + x1 ⋅ i, y0 + y1 ⋅ i).
    /// @notice The point at infinity is encoded as (0,0,0,0) and compressed to (0,0).
    /// @param x0 The real part of the X coordinate.
    /// @param x1 The imaginary poart of the X coordinate.
    /// @param y0 The real part of the Y coordinate.
    /// @param y1 The imaginary part of the Y coordinate.
    /// @return c0 The first half of the compresed point (x0 with two signal bits).
    /// @return c1 The second half of the compressed point (x1 unmodified).
    function compress_g2(
        uint256 x0,
        uint256 x1,
        uint256 y0,
        uint256 y1
    ) internal view returns (uint256 c0, uint256 c1) {
        if (x0 >= P || x1 >= P || y0 >= P || y1 >= P) {
            // G2 point not in field.
            revert ProofInvalid();
        }
        if ((x0 | x1 | y0 | y1) == 0) {
            // Point at infinity
            return (0, 0);
        }

        // Compute y^2
        // Note: shadowing variables and scoping to avoid stack-to-deep.
        uint256 y0_pos;
        uint256 y1_pos;
        {
            uint256 n3ab = mulmod(mulmod(x0, x1, P), P - 3, P);
            uint256 a_3 = mulmod(mulmod(x0, x0, P), x0, P);
            uint256 b_3 = mulmod(mulmod(x1, x1, P), x1, P);
            y0_pos = addmod(
                FRACTION_27_82_FP,
                addmod(a_3, mulmod(n3ab, x1, P), P),
                P
            );
            y1_pos = negate(
                addmod(FRACTION_3_82_FP, addmod(b_3, mulmod(n3ab, x0, P), P), P)
            );
        }

        // Determine hint bit
        // If this sqrt fails the x coordinate is not on the curve.
        bool hint;
        {
            uint256 d = sqrt_Fp(
                addmod(mulmod(y0_pos, y0_pos, P), mulmod(y1_pos, y1_pos, P), P)
            );
            hint = !isSquare_Fp(
                mulmod(addmod(y0_pos, d, P), FRACTION_1_2_FP, P)
            );
        }

        // Recover y
        (y0_pos, y1_pos) = sqrt_Fp2(y0_pos, y1_pos, hint);
        if (y0 == y0_pos && y1 == y1_pos) {
            c0 = (x0 << 2) | (hint ? 2 : 0) | 0;
            c1 = x1;
        } else if (y0 == negate(y0_pos) && y1 == negate(y1_pos)) {
            c0 = (x0 << 2) | (hint ? 2 : 0) | 1;
            c1 = x1;
        } else {
            // G1 point not on curve.
            revert ProofInvalid();
        }
    }

    /// Decompress a G2 point.
    /// @notice Reverts with InvalidProof if the input does not represent a valid point.
    /// @notice The G2 curve is defined over the complex extension Fp[i]/(i^2 + 1)
    /// with coordinates (x0 + x1 ⋅ i, y0 + y1 ⋅ i).
    /// @notice The point at infinity is encoded as (0,0,0,0) and compressed to (0,0).
    /// @param c0 The first half of the compresed point (x0 with two signal bits).
    /// @param c1 The second half of the compressed point (x1 unmodified).
    /// @return x0 The real part of the X coordinate.
    /// @return x1 The imaginary poart of the X coordinate.
    /// @return y0 The real part of the Y coordinate.
    /// @return y1 The imaginary part of the Y coordinate.
    function decompress_g2(
        uint256 c0,
        uint256 c1
    ) internal view returns (uint256 x0, uint256 x1, uint256 y0, uint256 y1) {
        // Note that X = (0, 0) is not on the curve since 0³ + 3/(9 + i) is not a square.
        // so we can use it to represent the point at infinity.
        if (c0 == 0 && c1 == 0) {
            // Point at infinity as encoded in EIP197.
            return (0, 0, 0, 0);
        }
        bool negate_point = c0 & 1 == 1;
        bool hint = c0 & 2 == 2;
        x0 = c0 >> 2;
        x1 = c1;
        if (x0 >= P || x1 >= P) {
            // G2 x0 or x1 coefficient not in field.
            revert ProofInvalid();
        }

        uint256 n3ab = mulmod(mulmod(x0, x1, P), P - 3, P);
        uint256 a_3 = mulmod(mulmod(x0, x0, P), x0, P);
        uint256 b_3 = mulmod(mulmod(x1, x1, P), x1, P);

        y0 = addmod(FRACTION_27_82_FP, addmod(a_3, mulmod(n3ab, x1, P), P), P);
        y1 = negate(
            addmod(FRACTION_3_82_FP, addmod(b_3, mulmod(n3ab, x0, P), P), P)
        );

        // Note: sqrt_Fp2 reverts if there is no solution, i.e. the point is not on the curve.
        // Note: (X³ + 3/(9 + i)) is irreducible in Fp2, so y can not be zero.
        //       But y0 or y1 may still independently be zero.
        (y0, y1) = sqrt_Fp2(y0, y1, hint);
        if (negate_point) {
            y0 = negate(y0);
            y1 = negate(y1);
        }
    }

    /// Compute the public input linear combination.
    /// @notice Reverts with PublicInputNotInField if the input is not in the field.
    /// @notice Computes the multi-scalar-multiplication of the public input
    /// elements and the verification key including the constant term.
    /// @param input The public inputs. These are elements of the scalar field Fr.
    /// @return x The X coordinate of the resulting G1 point.
    /// @return y The Y coordinate of the resulting G1 point.
    function publicInputMSM(
        uint256[32] calldata input
    ) internal view returns (uint256 x, uint256 y) {
        // Note: The ECMUL precompile does not reject unreduced values, so we check this.
        // Note: Unrolling this loop does not cost much extra in code-size, the bulk of the
        //       code-size is in the PUB_ constants.
        // ECMUL has input (x, y, scalar) and output (x', y').
        // ECADD has input (x1, y1, x2, y2) and output (x', y').
        // We reduce commitments(if any) with constants as the first point argument to ECADD.
        // We call them such that ecmul output is already in the second point
        // argument to ECADD so we can have a tight loop.
        bool success = true;
        assembly ("memory-safe") {
            let f := mload(0x40)
            let g := add(f, 0x40)
            let s
            mstore(f, CONSTANT_X)
            mstore(add(f, 0x20), CONSTANT_Y)
            mstore(g, PUB_0_X)
            mstore(add(g, 0x20), PUB_0_Y)
            s := calldataload(input)
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_1_X)
            mstore(add(g, 0x20), PUB_1_Y)
            s := calldataload(add(input, 32))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_2_X)
            mstore(add(g, 0x20), PUB_2_Y)
            s := calldataload(add(input, 64))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_3_X)
            mstore(add(g, 0x20), PUB_3_Y)
            s := calldataload(add(input, 96))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_4_X)
            mstore(add(g, 0x20), PUB_4_Y)
            s := calldataload(add(input, 128))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_5_X)
            mstore(add(g, 0x20), PUB_5_Y)
            s := calldataload(add(input, 160))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_6_X)
            mstore(add(g, 0x20), PUB_6_Y)
            s := calldataload(add(input, 192))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_7_X)
            mstore(add(g, 0x20), PUB_7_Y)
            s := calldataload(add(input, 224))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_8_X)
            mstore(add(g, 0x20), PUB_8_Y)
            s := calldataload(add(input, 256))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_9_X)
            mstore(add(g, 0x20), PUB_9_Y)
            s := calldataload(add(input, 288))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_10_X)
            mstore(add(g, 0x20), PUB_10_Y)
            s := calldataload(add(input, 320))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_11_X)
            mstore(add(g, 0x20), PUB_11_Y)
            s := calldataload(add(input, 352))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_12_X)
            mstore(add(g, 0x20), PUB_12_Y)
            s := calldataload(add(input, 384))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_13_X)
            mstore(add(g, 0x20), PUB_13_Y)
            s := calldataload(add(input, 416))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_14_X)
            mstore(add(g, 0x20), PUB_14_Y)
            s := calldataload(add(input, 448))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_15_X)
            mstore(add(g, 0x20), PUB_15_Y)
            s := calldataload(add(input, 480))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_16_X)
            mstore(add(g, 0x20), PUB_16_Y)
            s := calldataload(add(input, 512))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_17_X)
            mstore(add(g, 0x20), PUB_17_Y)
            s := calldataload(add(input, 544))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_18_X)
            mstore(add(g, 0x20), PUB_18_Y)
            s := calldataload(add(input, 576))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_19_X)
            mstore(add(g, 0x20), PUB_19_Y)
            s := calldataload(add(input, 608))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_20_X)
            mstore(add(g, 0x20), PUB_20_Y)
            s := calldataload(add(input, 640))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_21_X)
            mstore(add(g, 0x20), PUB_21_Y)
            s := calldataload(add(input, 672))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_22_X)
            mstore(add(g, 0x20), PUB_22_Y)
            s := calldataload(add(input, 704))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_23_X)
            mstore(add(g, 0x20), PUB_23_Y)
            s := calldataload(add(input, 736))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_24_X)
            mstore(add(g, 0x20), PUB_24_Y)
            s := calldataload(add(input, 768))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_25_X)
            mstore(add(g, 0x20), PUB_25_Y)
            s := calldataload(add(input, 800))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_26_X)
            mstore(add(g, 0x20), PUB_26_Y)
            s := calldataload(add(input, 832))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_27_X)
            mstore(add(g, 0x20), PUB_27_Y)
            s := calldataload(add(input, 864))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_28_X)
            mstore(add(g, 0x20), PUB_28_Y)
            s := calldataload(add(input, 896))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_29_X)
            mstore(add(g, 0x20), PUB_29_Y)
            s := calldataload(add(input, 928))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_30_X)
            mstore(add(g, 0x20), PUB_30_Y)
            s := calldataload(add(input, 960))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )
            mstore(g, PUB_31_X)
            mstore(add(g, 0x20), PUB_31_Y)
            s := calldataload(add(input, 992))
            mstore(add(g, 0x40), s)
            success := and(success, lt(s, R))
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_MUL, g, 0x60, g, 0x40)
            )
            success := and(
                success,
                staticcall(gas(), PRECOMPILE_ADD, f, 0x80, f, 0x40)
            )

            x := mload(f)
            y := mload(add(f, 0x20))
        }
        if (!success) {
            // Either Public input not in field, or verification key invalid.
            // We assume the contract is correctly generated, so the verification key is valid.
            revert PublicInputNotInField();
        }
    }

    /// Compress a proof.
    /// @notice Will revert with InvalidProof if the curve points are invalid,
    /// but does not verify the proof itself.
    /// @param proof The uncompressed Groth16 proof. Elements are in the same order as for
    /// verifyProof. I.e. Groth16 points (A, B, C) encoded as in EIP-197.
    /// @return compressed The compressed proof. Elements are in the same order as for
    /// verifyCompressedProof. I.e. points (A, B, C) in compressed format.
    function compressProof(
        uint256[8] calldata proof
    ) public view returns (uint256[4] memory compressed) {
        compressed[0] = compress_g1(proof[0], proof[1]);
        (compressed[2], compressed[1]) = compress_g2(
            proof[3],
            proof[2],
            proof[5],
            proof[4]
        );
        compressed[3] = compress_g1(proof[6], proof[7]);
    }

    /// Verify a Groth16 proof with compressed points.
    /// @notice Reverts with InvalidProof if the proof is invalid or
    /// with PublicInputNotInField the public input is not reduced.
    /// @notice There is no return value. If the function does not revert, the
    /// proof was successfully verified.
    /// @param compressedProof the points (A, B, C) in compressed format
    /// matching the output of compressProof.
    /// @param input the public input field elements in the scalar field Fr.
    /// Elements must be reduced.
    function verifyCompressedProof(
        uint256[4] calldata compressedProof,
        uint256[32] calldata input
    ) public view {
        uint256[24] memory pairings;

        {
            (uint256 Ax, uint256 Ay) = decompress_g1(compressedProof[0]);
            (
                uint256 Bx0,
                uint256 Bx1,
                uint256 By0,
                uint256 By1
            ) = decompress_g2(compressedProof[2], compressedProof[1]);
            (uint256 Cx, uint256 Cy) = decompress_g1(compressedProof[3]);
            (uint256 Lx, uint256 Ly) = publicInputMSM(input);

            // Verify the pairing
            // Note: The precompile expects the F2 coefficients in big-endian order.
            // Note: The pairing precompile rejects unreduced values, so we won't check that here.
            // e(A, B)
            pairings[0] = Ax;
            pairings[1] = Ay;
            pairings[2] = Bx1;
            pairings[3] = Bx0;
            pairings[4] = By1;
            pairings[5] = By0;
            // e(C, -δ)
            pairings[6] = Cx;
            pairings[7] = Cy;
            pairings[8] = DELTA_NEG_X_1;
            pairings[9] = DELTA_NEG_X_0;
            pairings[10] = DELTA_NEG_Y_1;
            pairings[11] = DELTA_NEG_Y_0;
            // e(α, -β)
            pairings[12] = ALPHA_X;
            pairings[13] = ALPHA_Y;
            pairings[14] = BETA_NEG_X_1;
            pairings[15] = BETA_NEG_X_0;
            pairings[16] = BETA_NEG_Y_1;
            pairings[17] = BETA_NEG_Y_0;
            // e(L_pub, -γ)
            pairings[18] = Lx;
            pairings[19] = Ly;
            pairings[20] = GAMMA_NEG_X_1;
            pairings[21] = GAMMA_NEG_X_0;
            pairings[22] = GAMMA_NEG_Y_1;
            pairings[23] = GAMMA_NEG_Y_0;

            // Check pairing equation.
            bool success;
            uint256[1] memory output;
            assembly ("memory-safe") {
                success := staticcall(
                    gas(),
                    PRECOMPILE_VERIFY,
                    pairings,
                    0x300,
                    output,
                    0x20
                )
            }
            if (!success || output[0] != 1) {
                // Either proof or verification key invalid.
                // We assume the contract is correctly generated, so the verification key is valid.
                revert ProofInvalid();
            }
        }
    }

    /// Verify an uncompressed Groth16 proof.
    /// @notice Reverts with InvalidProof if the proof is invalid or
    /// with PublicInputNotInField the public input is not reduced.
    /// @notice There is no return value. If the function does not revert, the
    /// proof was successfully verified.
    /// @param proof the points (A, B, C) in EIP-197 format matching the output
    /// of compressProof.
    /// @param input the public input field elements in the scalar field Fr.
    /// Elements must be reduced.
    function verifyProof(
        uint256[8] calldata proof,
        uint256[32] calldata input
    ) public view {
        (uint256 x, uint256 y) = publicInputMSM(input);

        // Note: The precompile expects the F2 coefficients in big-endian order.
        // Note: The pairing precompile rejects unreduced values, so we won't check that here.
        bool success;
        assembly ("memory-safe") {
            let f := mload(0x40) // Free memory pointer.

            // Copy points (A, B, C) to memory. They are already in correct encoding.
            // This is pairing e(A, B) and G1 of e(C, -δ).
            calldatacopy(f, proof, 0x100)

            // Complete e(C, -δ) and write e(α, -β), e(L_pub, -γ) to memory.
            // OPT: This could be better done using a single codecopy, but
            //      Solidity (unlike standalone Yul) doesn't provide a way to
            //      to do this.
            mstore(add(f, 0x100), DELTA_NEG_X_1)
            mstore(add(f, 0x120), DELTA_NEG_X_0)
            mstore(add(f, 0x140), DELTA_NEG_Y_1)
            mstore(add(f, 0x160), DELTA_NEG_Y_0)
            mstore(add(f, 0x180), ALPHA_X)
            mstore(add(f, 0x1a0), ALPHA_Y)
            mstore(add(f, 0x1c0), BETA_NEG_X_1)
            mstore(add(f, 0x1e0), BETA_NEG_X_0)
            mstore(add(f, 0x200), BETA_NEG_Y_1)
            mstore(add(f, 0x220), BETA_NEG_Y_0)
            mstore(add(f, 0x240), x)
            mstore(add(f, 0x260), y)
            mstore(add(f, 0x280), GAMMA_NEG_X_1)
            mstore(add(f, 0x2a0), GAMMA_NEG_X_0)
            mstore(add(f, 0x2c0), GAMMA_NEG_Y_1)
            mstore(add(f, 0x2e0), GAMMA_NEG_Y_0)

            // Check pairing equation.
            success := staticcall(gas(), PRECOMPILE_VERIFY, f, 0x300, f, 0x20)
            // Also check returned value (both are either 1 or 0).
            success := and(success, mload(f))
        }
        if (!success) {
            // Either proof or verification key invalid.
            // We assume the contract is correctly generated, so the verification key is valid.
            revert ProofInvalid();
        }
    }
}
