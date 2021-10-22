// Copyright 2020-Present VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"
)

func newImageAccessError(ref string, err error) error {
	if transportError, ok := err.(*transport.Error); ok {
		if transportError.StatusCode == 401 {
			return errors.Errorf("invalid credentials, ensure registry credentials for '%s' are available", ref)
		}
	}
	return errors.WithStack(err)
}
