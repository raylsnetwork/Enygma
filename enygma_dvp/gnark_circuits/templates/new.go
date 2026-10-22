package templates

import "github.com/consensys/gnark/frontend"

// NewPaymentCircuit returns a PaymentCircuit with Config set and every slice
// sized for cfg, ready to compile or to fill in as a witness. The statement
// layouts build their circuits through these New*Circuit functions, so every
// layout has the shape the circuit compiles to at cfg.
func NewPaymentCircuit(cfg PaymentCircuitConfig) PaymentCircuit {
	return PaymentCircuit{
		Config:               cfg,
		StTreeNumbers:        variables(cfg.TmNInputs),
		StMerkleRoots:        variables(cfg.TmNInputs),
		StNullifiers:         variables(cfg.TmNInputs),
		StCommitmentsOut:     variables(cfg.TmMOutputs),
		WtPrivateKeysIn:      variables(cfg.TmNInputs),
		WtValuesIn:           variables(cfg.TmNInputs),
		WtSaltsIn:            variables(cfg.TmNInputs),
		WtPathElements:       paths(cfg),
		WtPathIndices:        variables(cfg.TmNInputs),
		WtSpendPublicKeysOut: variables(cfg.TmMOutputs),
		WtValuesOut:          variables(cfg.TmMOutputs),
		WtSaltsOut:           variables(cfg.TmMOutputs),
	}
}

// NewPaymentFeeCircuit is NewPaymentCircuit for PaymentFeeCircuit.
func NewPaymentFeeCircuit(cfg PaymentCircuitConfig) PaymentFeeCircuit {
	return PaymentFeeCircuit{
		Config:               cfg,
		StTreeNumbers:        variables(cfg.TmNInputs),
		StMerkleRoots:        variables(cfg.TmNInputs),
		StNullifiers:         variables(cfg.TmNInputs),
		StCommitmentsOut:     variables(cfg.TmMOutputs),
		WtPrivateKeysIn:      variables(cfg.TmNInputs),
		WtValuesIn:           variables(cfg.TmNInputs),
		WtSaltsIn:            variables(cfg.TmNInputs),
		WtPathElements:       paths(cfg),
		WtPathIndices:        variables(cfg.TmNInputs),
		WtSpendPublicKeysOut: variables(cfg.TmMOutputs),
		WtValuesOut:          variables(cfg.TmMOutputs),
		WtSaltsOut:           variables(cfg.TmMOutputs),
	}
}

// NewPaymentRelayerFeePublicCircuit is NewPaymentCircuit for PaymentRelayerFeePublicCircuit.
func NewPaymentRelayerFeePublicCircuit(cfg PaymentCircuitConfig) PaymentRelayerFeePublicCircuit {
	return PaymentRelayerFeePublicCircuit{
		Config:               cfg,
		StTreeNumbers:        variables(cfg.TmNInputs),
		StMerkleRoots:        variables(cfg.TmNInputs),
		StNullifiers:         variables(cfg.TmNInputs),
		StCommitmentsOut:     variables(cfg.TmMOutputs),
		WtPrivateKeysIn:      variables(cfg.TmNInputs),
		WtValuesIn:           variables(cfg.TmNInputs),
		WtSaltsIn:            variables(cfg.TmNInputs),
		WtPathElements:       paths(cfg),
		WtPathIndices:        variables(cfg.TmNInputs),
		WtSpendPublicKeysOut: variables(cfg.TmMOutputs),
		WtValuesOut:          variables(cfg.TmMOutputs),
		WtSaltsOut:           variables(cfg.TmMOutputs),
	}
}

// NewUsdrFeeCircuit is NewPaymentCircuit for UsdrFeeCircuit.
func NewUsdrFeeCircuit(cfg PaymentCircuitConfig) UsdrFeeCircuit {
	return UsdrFeeCircuit{
		Config:               cfg,
		StTreeNumbers:        variables(cfg.TmNInputs),
		StMerkleRoots:        variables(cfg.TmNInputs),
		StNullifiers:         variables(cfg.TmNInputs),
		StCommitmentsOut:     variables(cfg.TmMOutputs),
		WtPrivateKeysIn:      variables(cfg.TmNInputs),
		WtValuesIn:           variables(cfg.TmNInputs),
		WtSaltsIn:            variables(cfg.TmNInputs),
		WtPathElements:       paths(cfg),
		WtPathIndices:        variables(cfg.TmNInputs),
		WtSpendPublicKeysOut: variables(cfg.TmMOutputs),
		WtValuesOut:          variables(cfg.TmMOutputs),
		WtSaltsOut:           variables(cfg.TmMOutputs),
	}
}

func variables(n int) []frontend.Variable { return make([]frontend.Variable, n) }

// paths returns TmNInputs Merkle paths of TmMerkleTreeDepth elements.
func paths(cfg PaymentCircuitConfig) [][]frontend.Variable {
	p := make([][]frontend.Variable, cfg.TmNInputs)
	for i := range p {
		p[i] = variables(cfg.TmMerkleTreeDepth)
	}
	return p
}
