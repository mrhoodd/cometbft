package types

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/v2/crypto"
	"github.com/cometbft/cometbft/v2/crypto/batch"
	"github.com/cometbft/cometbft/v2/crypto/tmhash"
	cmtmath "github.com/cometbft/cometbft/v2/libs/math"
	cmterrors "github.com/cometbft/cometbft/v2/types/errors"
)

const batchVerifyThreshold = 2

func shouldBatchVerify(vals *ValidatorSet, commit *Commit) bool {
	return len(commit.Signatures) >= batchVerifyThreshold &&
		batch.SupportsBatchVerifier(vals.GetProposer().PubKey) &&
		vals.AllKeysHaveSameType()
}

// VerifyCommit verifies +2/3 of the set had signed the given commit.
//
// It checks all the signatures! While it's safe to exit as soon as we have
// 2/3+ signatures, doing so would impact incentivization logic in the ABCI
// application that depends on the LastCommitInfo sent in FinalizeBlock, which
// includes which validators signed. For instance, Gaia incentivizes proposers
// with a bonus for including more than +2/3 of the signatures.
func VerifyCommit(chainID string, vals *ValidatorSet, blockID BlockID,
	height int64, commit *Commit,
) error {
	// run a basic validation of the arguments
	if err := verifyBasicValsAndCommit(vals, commit, height, blockID); err != nil {
		return err
	}

	// calculate voting power needed. Note that total voting power is capped to
	// 1/8th of max int64 so this operation should never overflow
	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3

	// ignore all absent signatures
	ignore := func(c CommitSig) bool { return c.BlockIDFlag == BlockIDFlagAbsent }

	// only count the signatures that are for the block
	count := func(c CommitSig) bool { return c.BlockIDFlag == BlockIDFlagCommit }

	// attempt to batch verify
	if shouldBatchVerify(vals, commit) {
		return verifyCommitBatch(chainID, vals, commit,
			votingPowerNeeded, ignore, count, true, true, nil, nil)
	}

	// if verification failed or is not supported then fallback to single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		ignore, count, true, true, nil)
}

// LIGHT CLIENT VERIFICATION METHODS

// VerifyCommitLight verifies +2/3 of the set had signed the given commit.
//
// This method is primarily used by the light client and does NOT check all the
// signatures.
func VerifyCommitLight(
	chainID string,
	vals *ValidatorSet,
	blockID BlockID,
	height int64,
	commit *Commit,
) error {
	return verifyCommitLightInternal(chainID, vals, blockID, height, commit, false, nil)
}

// VerifyCommitLightWithCache verifies +2/3 of the set had signed the given commit.
//
// This method is primarily used by the light client and does NOT check all the
// signatures.
// The cache provided will be used to skip signature verification for entries where the
// key (signature), validator pubkey, and vote sign bytes all match.
// Additionally, any verified signatures will be added to the cache.
func VerifyCommitLightWithCache(
	chainID string,
	vals *ValidatorSet,
	blockID BlockID,
	height int64,
	commit *Commit,
	verifiedSignatureCache SignatureCache,
) error {
	return verifyCommitLightInternal(chainID, vals, blockID, height, commit, false, verifiedSignatureCache)
}

// VerifyCommitLightAllSignatures verifies +2/3 of the set had signed the given commit.
//
// This method DOES check all the signatures.
func VerifyCommitLightAllSignatures(
	chainID string,
	vals *ValidatorSet,
	blockID BlockID,
	height int64,
	commit *Commit,
) error {
	return verifyCommitLightInternal(chainID, vals, blockID, height, commit, true, nil)
}

func verifyCommitLightInternal(
	chainID string,
	vals *ValidatorSet,
	blockID BlockID,
	height int64,
	commit *Commit,
	countAllSignatures bool,
	verifiedSignatureCache SignatureCache,
) error {
	// run a basic validation of the arguments
	if err := verifyBasicValsAndCommit(vals, commit, height, blockID); err != nil {
		return err
	}

	// calculate voting power needed
	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3

	// ignore all commit signatures that are not for the block
	ignore := func(c CommitSig) bool { return c.BlockIDFlag != BlockIDFlagCommit }

	// count all the remaining signatures
	count := func(_ CommitSig) bool { return true }

	// attempt to batch verify
	if shouldBatchVerify(vals, commit) {
		return verifyCommitBatch(chainID, vals, commit,
			votingPowerNeeded, ignore, count, countAllSignatures, true, nil, verifiedSignatureCache)
	}

	// if verification failed or is not supported then fallback to single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		ignore, count, countAllSignatures, true, verifiedSignatureCache)
}

// VerifyCommitLightTrusting verifies that trustLevel of the validator set signed
// this commit. "Trusting" means that we trust the validator set to be correct.
//
// NOTE the given validators do not necessarily correspond to the validator set
// for this commit, but there may be some intersection.
//
// This method is primarily used by the light client and does NOT check all the
// signatures.
//
// CONTRACT: must run ValidateBasic() on commit before verifying.
func VerifyCommitLightTrusting(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	trustLevel cmtmath.Fraction,
) error {
	return verifyCommitLightTrustingInternal(chainID, vals, commit, trustLevel, false, nil)
}

// VerifyCommitLightTrustingWithCache verifies that trustLevel of the validator set signed
// this commit. "Trusting" means that we trust the validator set to be correct.
//
// NOTE the given validators do not necessarily correspond to the validator set
// for this commit, but there may be some intersection.
//
// This method is primarily used by the light client and does NOT check all the
// signatures.
//
// CONTRACT: must run ValidateBasic() on commit before verifying.
// The cache provided will be used to skip signature verification for entries where the
// key (signature), validator pubkey, and vote sign bytes all match.
// Additionally, any verified signatures will be added to the cache.
func VerifyCommitLightTrustingWithCache(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	trustLevel cmtmath.Fraction,
	verifiedSignatureCache SignatureCache,
) error {
	return verifyCommitLightTrustingInternal(chainID, vals, commit, trustLevel, false, verifiedSignatureCache)
}

// VerifyCommitLightTrustingAllSignatures verifies that trustLevel of the validator
// set signed this commit. "Trusting" means that we trust the validator set to be correct.
//
// NOTE the given validators do not necessarily correspond to the validator set
// for this commit, but there may be some intersection.
//
// This method DOES check all the signatures.
//
// CONTRACT: must run ValidateBasic() on commit before verifying.
func VerifyCommitLightTrustingAllSignatures(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	trustLevel cmtmath.Fraction,
) error {
	return verifyCommitLightTrustingInternal(chainID, vals, commit, trustLevel, true, nil)
}

func verifyCommitLightTrustingInternal(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	trustLevel cmtmath.Fraction,
	countAllSignatures bool,
	verifiedSignatureCache SignatureCache,
) error {
	// sanity checks
	if vals == nil {
		return errors.New("nil validator set")
	}
	if trustLevel.Denominator == 0 {
		return errors.New("trustLevel has zero Denominator")
	}
	if commit == nil {
		return errors.New("nil commit")
	}

	// safely calculate voting power needed.
	totalVotingPowerMulByNumerator, overflow := safeMul(vals.TotalVotingPower(), int64(trustLevel.Numerator))
	if overflow {
		return errors.New("int64 overflow while calculating voting power needed. please provide smaller trustLevel numerator")
	}
	votingPowerNeeded := totalVotingPowerMulByNumerator / int64(trustLevel.Denominator)

	// ignore all commit signatures that are not for the block
	ignore := func(c CommitSig) bool { return c.BlockIDFlag != BlockIDFlagCommit }

	// count all the remaining signatures
	count := func(_ CommitSig) bool { return true }

	// attempt to batch verify commit. As the validator set doesn't necessarily
	// correspond with the validator set that signed the block we need to look
	// up by address rather than index.
	if shouldBatchVerify(vals, commit) {
		return verifyCommitBatch(chainID, vals, commit,
			votingPowerNeeded, ignore, count, countAllSignatures, false, nil, verifiedSignatureCache)
	}

	// attempt with single verification
	return verifyCommitSingle(chainID, vals, commit, votingPowerNeeded,
		ignore, count, countAllSignatures, false, verifiedSignatureCache)
}

// ValidateHash returns an error if the hash is not empty, but its
// size != tmhash.Size.
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != tmhash.Size {
		return fmt.Errorf("expected size to be %d bytes, got %d bytes",
			tmhash.Size,
			len(h),
		)
	}
	return nil
}

// Batch verification

// verifyCommitBatch batch verifies commits.  This routine is equivalent
// to verifyCommitSingle in behavior, just faster iff every signature in the
// batch is valid.
//
// Note: The caller is responsible for checking to see if this routine is
// usable via `shouldVerifyBatch(vals, commit)`.
func verifyCommitBatch(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	votingPowerNeeded int64,
	ignoreSig func(CommitSig) bool,
	countSig func(CommitSig) bool,
	countAllSignatures bool,
	lookUpByIndex bool,
	batchVerifier crypto.BatchVerifier,
	verifiedSignatureCache SignatureCache,
) error {
	var (
		val                *Validator
		valIdx             int32
		seenVals           = make(map[int32]int, len(commit.Signatures))
		batchSigIdxs       = make([]int, 0, len(commit.Signatures))
		talliedVotingPower int64
	)
	// attempt to create a batch verifier
	bv, ok := batchVerifier, true
	if batchVerifier == nil {
		bv, ok = batch.CreateBatchVerifier(vals.GetProposer().PubKey)
	}
	// re-check if batch verification is supported
	if !ok || len(commit.Signatures) < batchVerifyThreshold {
		// This should *NEVER* happen.
		return errors.New("unsupported signature algorithm or insufficient signatures for batch verification")
	}

	for idx, commitSig := range commit.Signatures {
		// skip over signatures that should be ignored
		if ignoreSig(commitSig) {
			continue
		}

		// If the vals and commit have a 1-to-1 correspondence we can retrieve
		// them by index else we need to retrieve them by address
		if lookUpByIndex {
			val = vals.Validators[idx]
		} else {
			valIdx, val = vals.GetByAddressMut(commitSig.ValidatorAddress)

			// if the signature doesn't belong to anyone in the validator set
			// then we just skip over it
			if val == nil {
				continue
			}

			// because we are getting validators by address we need to make sure
			// that the same validator doesn't commit twice
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx
		}

		// Validate signature.
		voteSignBytes := commit.VoteSignBytes(chainID, int32(idx))

		cacheHit := false
		if verifiedSignatureCache != nil {
			cacheVal, sigIsInCache := verifiedSignatureCache.Get(string(commitSig.Signature))
			cacheHit = sigIsInCache && bytes.Equal(cacheVal.ValidatorAddress, val.PubKey.Address()) && bytes.Equal(cacheVal.VoteSignBytes, voteSignBytes)
		}

		if !cacheHit {
			// add the key, sig and message to the verifier
			if err := bv.Add(val.PubKey, voteSignBytes, commitSig.Signature); err != nil {
				return err
			}
			batchSigIdxs = append(batchSigIdxs, idx)
		}

		// If this signature counts then add the voting power of the validator
		// to the tally
		if countSig(commitSig) {
			talliedVotingPower += val.VotingPower
		}

		// if we don't need to verify all signatures and already have sufficient
		// voting power we can break from batching and verify all the signatures
		if !countAllSignatures && talliedVotingPower > votingPowerNeeded {
			break
		}
	}

	// ensure that we have batched together enough signatures to exceed the
	// voting power needed else there is no need to even verify
	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	// if every signature was in the cache, the batch verifier is empty and we shouldn't call verify
	if len(batchSigIdxs) == 0 {
		return nil
	}

	// attempt to verify the batch.
	ok, validSigs := bv.Verify()
	if ok {
		// success
		if verifiedSignatureCache != nil {
			for i := range validSigs {
				idx := batchSigIdxs[i]
				sig := commit.Signatures[idx]
				verifiedSignatureCache.Add(string(sig.Signature), SignatureCacheValue{
					ValidatorAddress: sig.ValidatorAddress,
					VoteSignBytes:    commit.VoteSignBytes(chainID, int32(idx)),
				})
			}
		}

		return nil
	}

	// one or more of the signatures is invalid, find and return the first
	// invalid signature.
	for i, ok := range validSigs {
		// go back from the batch index to the commit.Signatures index
		idx := batchSigIdxs[i]
		sig := commit.Signatures[idx]
		if !ok {
			return fmt.Errorf("wrong signature (#%d): %X", idx, sig)
		}
		if verifiedSignatureCache != nil {
			verifiedSignatureCache.Add(string(sig.Signature), SignatureCacheValue{
				ValidatorAddress: sig.ValidatorAddress,
				VoteSignBytes:    commit.VoteSignBytes(chainID, int32(idx)),
			})
		}
	}

	// execution reaching here is a bug, and one of the following has
	// happened:
	//  * non-zero tallied voting power, empty batch (impossible?)
	//  * bv.Verify() returned `false, []bool{true, ..., true}` (BUG)
	return errors.New("BUG: batch verification failed with no invalid signatures")
}

// Single Verification

// verifyCommitSingle single verifies commits.
// If a key does not support batch verification, or batch verification fails this will be used
// This method is used to check all the signatures included in a commit.
// It is used in consensus for validating a block LastCommit.
// CONTRACT: both commit and validator set should have passed validate basic.
func verifyCommitSingle(
	chainID string,
	vals *ValidatorSet,
	commit *Commit,
	votingPowerNeeded int64,
	ignoreSig func(CommitSig) bool,
	countSig func(CommitSig) bool,
	countAllSignatures bool,
	lookUpByIndex bool,
	verifiedSignatureCache SignatureCache,
) error {
	var (
		val                *Validator
		valIdx             int32
		seenVals           = make(map[int32]int, len(commit.Signatures))
		talliedVotingPower int64
		voteSignBytes      []byte
	)
	for idx, commitSig := range commit.Signatures {
		if ignoreSig(commitSig) {
			continue
		}

		if commitSig.ValidateBasic() != nil {
			return fmt.Errorf("invalid signatures from %v at index %d", val, idx)
		}

		// If the vals and commit have a 1-to-1 correspondence we can retrieve
		// them by index else we need to retrieve them by address
		if lookUpByIndex {
			val = vals.Validators[idx]
		} else {
			valIdx, val = vals.GetByAddress(commitSig.ValidatorAddress)

			// if the signature doesn't belong to anyone in the validator set
			// then we just skip over it
			if val == nil {
				continue
			}

			// because we are getting validators by address we need to make sure
			// that the same validator doesn't commit twice
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx
		}

		if val.PubKey == nil {
			return fmt.Errorf("validator %v has a nil PubKey at index %d", val, idx)
		}

		voteSignBytes = commit.VoteSignBytes(chainID, int32(idx))

		cacheKey, cacheHit := "", false
		if verifiedSignatureCache != nil {
			cacheKey = string(commitSig.Signature)
			cacheVal, sigIsInCache := verifiedSignatureCache.Get(cacheKey)
			cacheHit = sigIsInCache && bytes.Equal(cacheVal.ValidatorAddress, val.PubKey.Address()) && bytes.Equal(cacheVal.VoteSignBytes, voteSignBytes)
		}

		if !cacheHit {
			if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
				return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
			}
			if verifiedSignatureCache != nil {
				verifiedSignatureCache.Add(cacheKey, SignatureCacheValue{
					ValidatorAddress: val.PubKey.Address(),
					VoteSignBytes:    voteSignBytes,
				})
			}
		}

		// If this signature counts then add the voting power of the validator
		// to the tally
		if countSig(commitSig) {
			talliedVotingPower += val.VotingPower
		}

		// check if we have enough signatures and can thus exit early
		if !countAllSignatures && talliedVotingPower > votingPowerNeeded {
			return nil
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	return nil
}

func verifyBasicValsAndCommit(vals *ValidatorSet, commit *Commit, height int64, blockID BlockID) error {
	if vals == nil {
		return errors.New("nil validator set")
	}

	if commit == nil {
		return errors.New("nil commit")
	}

	if vals.Size() != len(commit.Signatures) {
		return cmterrors.NewErrInvalidCommitSignatures(vals.Size(), len(commit.Signatures))
	}

	// Validate Height and BlockID.
	if height != commit.Height {
		return cmterrors.NewErrInvalidCommitHeight(height, commit.Height)
	}
	if !blockID.Equals(commit.BlockID) {
		return fmt.Errorf("invalid commit -- wrong block ID: want %v, got %v",
			blockID, commit.BlockID)
	}

	return nil
}
