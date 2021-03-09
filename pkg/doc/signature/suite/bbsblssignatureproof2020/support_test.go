/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bbsblssignatureproof2020_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/piprate/json-gold/ld"

	"github.com/hyperledger/aries-framework-go/pkg/doc/jsonld"
)

const jsonldContextPrefix = "testdata/context"

func addJSONLDCachedContextFromFile(loader *jsonld.CachingDocumentLoader, contextURL, contextFile string) {
	contextContent, err := ioutil.ReadFile(filepath.Clean(filepath.Join(
		jsonldContextPrefix, contextFile)))
	if err != nil {
		panic(err)
	}

	addJSONLDCachedContext(loader, contextURL, string(contextContent))
}

func createLDPBBS2020DocumentLoader() ld.DocumentLoader {
	loader := jsonld.NewDefaultCachingDocumentLoader()

	addJSONLDCachedContextFromFile(loader,
		"https://www.w3.org/2018/credentials/v1", "vc.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://www.w3.org/2018/credentials/examples/v1", "vc-examples.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://www.w3.org/ns/odrl.jsonld", "ordl.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://w3id.org/security/bbs/v1", "ldp-bbs2020.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://w3id.org/security/v1", "security_v1.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://w3id.org/security/v2", "security_v2.jsonld")

	addJSONLDCachedContextFromFile(loader,
		"https://w3id.org/citizenship/v1", "citizenship.jsonld")

	return loader
}

func addJSONLDCachedContext(loader *jsonld.CachingDocumentLoader, contextURL, contextContent string) {
	reader, err := ld.DocumentFromReader(strings.NewReader(contextContent))
	if err != nil {
		panic(err)
	}

	loader.AddDocument(contextURL, reader)
}
