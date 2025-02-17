// Copyright 2021 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package predicate

import (
	"context"
	"fmt"

	"github.com/palantir/policy-bot/policy/common"
	"github.com/palantir/policy-bot/pull"
	"github.com/pkg/errors"
)

type HasValidSignatures bool

var _ Predicate = HasValidSignatures(false)

func (pred HasValidSignatures) Evaluate(ctx context.Context, prctx pull.Context) (bool, string, error) {
	commits, err := prctx.Commits()
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get commits")
	}

	for _, c := range commits {
		valid, desc := hasValidSignature(ctx, c)
		if !valid {
			if pred {
				return false, desc, nil
			}
			return true, "", nil
		}
	}

	if pred {
		return true, "", nil
	}
	return false, "All commits are signed and have valid signatures", nil
}

func (pred HasValidSignatures) Trigger() common.Trigger {
	return common.TriggerCommit
}

type HasValidSignaturesBy struct {
	common.Actors `yaml:",inline"`
}

var _ Predicate = &HasValidSignaturesBy{}

func (pred *HasValidSignaturesBy) Evaluate(ctx context.Context, prctx pull.Context) (bool, string, error) {
	commits, err := prctx.Commits()
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get commits")
	}

	signers := make(map[string]struct{})

	for _, c := range commits {
		valid, desc := hasValidSignature(ctx, c)
		if !valid {
			return false, desc, nil
		}
		signers[c.Signature.Signer] = struct{}{}
	}

	for signer := range signers {
		member, err := pred.IsActor(ctx, prctx, signer)
		if err != nil {
			return false, "", err
		}
		if !member {
			return false, fmt.Sprintf("Contributor %q does not meet the required membership conditions for signing", signer), nil
		}
	}

	return true, "", nil
}

func (pred *HasValidSignaturesBy) Trigger() common.Trigger {
	return common.TriggerCommit
}

type HasValidSignaturesByKeys struct {
	KeyIDs []string `yaml:"key_ids"`
}

var _ Predicate = &HasValidSignaturesByKeys{}

func (pred *HasValidSignaturesByKeys) Evaluate(ctx context.Context, prctx pull.Context) (bool, string, error) {
	commits, err := prctx.Commits()
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get commits")
	}

	keys := make(map[string]struct{})

	for _, c := range commits {
		valid, desc := hasValidSignature(ctx, c)
		if !valid {
			return false, desc, nil
		}
		// Only GPG signatures are valid for this predicate
		switch c.Signature.Type {
		case pull.SignatureGpg:
			keys[c.Signature.KeyID] = struct{}{}
		default:
			return false, fmt.Sprintf("Commit %.10s signature is not a GPG signature", c.SHA), nil
		}
	}

	for key := range keys {
		isValidKey := false
		for _, acceptedKey := range pred.KeyIDs {
			if key == acceptedKey {
				isValidKey = true
				break
			}
		}
		if !isValidKey {
			return false, fmt.Sprintf("Key %q does not meet the required key conditions for signing", key), nil
		}
	}

	return true, "", nil
}

func (pred *HasValidSignaturesByKeys) Trigger() common.Trigger {
	return common.TriggerCommit
}

func hasValidSignature(ctx context.Context, commit *pull.Commit) (bool, string) {
	if commit.Signature == nil {
		return false, fmt.Sprintf("Commit %.10s has no signature", commit.SHA)
	}
	if !commit.Signature.IsValid {
		reason := commit.Signature.State
		return false, fmt.Sprintf("Commit %.10s has an invalid signature due to %s", commit.SHA, reason)
	}
	return true, ""
}
