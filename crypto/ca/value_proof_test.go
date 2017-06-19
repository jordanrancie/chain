package ca

import "testing"

func TestValueProof(t *testing.T) {
	assetID := AssetID{1}
	value := uint64(2)
	aek := AssetKey{3}
	ac, c := CreateAssetCommitment(assetID, aek)
	vek := ValueKey{4}
	vc, f := CreateValueCommitment(value, ac, vek)
	msg := []byte("message")

	vp := CreateValueProof(assetID, value, ac, vc, *c, *f, msg)

	if !vp.Validate(assetID, value, ac, vc, msg) {
		t.Error("failed to validate value proof")
	}
	if vp.Validate(AssetID{1, 1}, value, ac, vc, msg) {
		t.Error("validated invalid value proof")
	}
	if vp.Validate(assetID, value+1, ac, vc, msg) {
		t.Error("validated invalid value proof")
	}
	if vp.Validate(assetID, value, ac, vc, msg[1:]) {
		t.Error("validated invalid value proof")
	}
	vp[0] ^= 1
	if vp.Validate(assetID, value, ac, vc, msg) {
		t.Error("validated invalid value proof")
	}
}
