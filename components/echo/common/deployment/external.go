// Copyright Istio Authors
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

package deployment

import (
	"path"
	"strconv"

	"github.com/jacob-delgado/integ-testing-framework/components/echo"
	"github.com/jacob-delgado/integ-testing-framework/components/echo/common/ports"
	"github.com/jacob-delgado/integ-testing-framework/components/echo/deployment"
	"github.com/jacob-delgado/integ-testing-framework/components/echo/match"
	"github.com/jacob-delgado/integ-testing-framework/components/namespace"
	"github.com/jacob-delgado/integ-testing-framework/echo/common"
	"github.com/jacob-delgado/integ-testing-framework/env"
	"github.com/jacob-delgado/integ-testing-framework/resource"
	"github.com/jacob-delgado/integ-testing-framework/util/file"
)

const (
	ExternalSvc      = "external"
	ExternalHostname = "fake.external.com"
)

type External struct {
	// Namespace where external echo app will be deployed
	Namespace namespace.Instance

	// All external echo instances with no sidecar injected
	All echo.Instances
}

func (e External) build(t resource.Context, b deployment.Builder) deployment.Builder {
	config := echo.Config{
		Service:           ExternalSvc,
		Namespace:         e.Namespace,
		DefaultHostHeader: ExternalHostname,
		Ports:             ports.All(),
		// Set up TLS certs on the server. This will make the server listen with these credentials.
		TLSSettings: &common.TLSSettings{
			// Echo has these test certs baked into the docker image
			RootCert:   file.MustAsString(path.Join(env.IstioSrc, "tests/testdata/certs/dns/root-cert.pem")),
			ClientCert: file.MustAsString(path.Join(env.IstioSrc, "tests/testdata/certs/dns/cert-chain.pem")),
			Key:        file.MustAsString(path.Join(env.IstioSrc, "tests/testdata/certs/dns/key.pem")),
			// Override hostname to match the SAN in the cert we are using
			// TODO(nmittler): We should probably make this the same as ExternalHostname
			Hostname: "server.default.svc",
		},
		Subsets: []echo.SubsetConfig{
			{
				Version: "v1",
				Annotations: map[echo.Annotation]*echo.AnnotationValue{
					echo.SidecarInject: {
						Value: strconv.FormatBool(false),
					},
				},
			},
		},
	}
	if t.Settings().EnableDualStack {
		config.IPFamilies = "IPv6, IPv4"
		config.IPFamilyPolicy = "RequireDualStack"
	}
	return b.WithConfig(config)
}

func (e *External) loadValues(echos echo.Instances) error {
	e.All = match.ServiceName(echo.NamespacedName{Name: ExternalSvc, Namespace: e.Namespace}).GetMatches(echos)
	return nil
}
