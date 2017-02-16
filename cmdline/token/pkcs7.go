/*
 * Copyright (c) SAS Institute Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package token

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"gerrit-pdt.unx.sas.com/tools/relic.git/cmdline/shared"
	"gerrit-pdt.unx.sas.com/tools/relic.git/config"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/certloader"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/pkcs7"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/pkcs9"
	"gerrit-pdt.unx.sas.com/tools/relic.git/p11token"
)

func readCerts(key *p11token.Key) ([]*x509.Certificate, error) {
	if key.X509Certificate == "" {
		return nil, p11token.ErrNoCertificate{"X509"}
	}
	certblob, err := ioutil.ReadFile(key.X509Certificate)
	if err != nil {
		return nil, err
	}
	return certloader.ParseCertificates(certblob)
}

func signAndTimestamp(data []byte, key *p11token.Key, opts crypto.SignerOpts, detach bool) (sig []byte, err error) {
	var psd *pkcs7.ContentInfoSignedData
	hash := opts.HashFunc()
	certs, err := readCerts(key)
	if err != nil {
		return nil, err
	}
	if detach {
		d := hash.New()
		d.Write(data)
		psd, err = pkcs7.SignDetached(d.Sum(nil), key, certs, opts)
	} else {
		psd, err = pkcs7.SignData(data, key, certs, opts)
	}
	if err != nil {
		return nil, err
	}
	return timestampPkcs(psd, key, certs, opts.HashFunc(), false)
}

func timestampPkcs(psd *pkcs7.ContentInfoSignedData, key *p11token.Key, certs []*x509.Certificate, hash crypto.Hash, authenticode bool) (sig []byte, err error) {
	keyConf := shared.CurrentConfig.Keys[key.Name]
	if keyConf.Timestamp {
		signerInfo := &psd.Content.SignerInfos[0]
		d := hash.New()
		d.Write(signerInfo.EncryptedDigest)
		imprint := d.Sum(nil)

		tconf, err := shared.CurrentConfig.GetTimestampConfig()
		if err != nil {
			return nil, fmt.Errorf("Unable to timestamp for key \"%s\": %s", key.Name, err)
		}
		cl := pkcs9.TimestampClient{
			UserAgent: config.UserAgent,
			CaFile:    tconf.CaCert,
			Timeout:   time.Second * time.Duration(tconf.Timeout),
		}
		var token *pkcs7.ContentInfoSignedData
		shuf := rand.New(rand.NewSource(time.Now().UnixNano()))
		order := shuf.Perm(len(tconf.Urls))
		for _, i := range order {
			cl.Url = tconf.Urls[i]
			if err != nil {
				fmt.Fprintf(os.Stderr, "Timestamping failed: %s\nTrying next server %s...\n", err, cl.Url)
			}
			token, err = cl.Request(hash, imprint)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("Timestamping failed: %s", err)
		}
		if authenticode {
			err = pkcs9.AddStampToSignedAuthenticode(signerInfo, *token)
		} else {
			err = pkcs9.AddStampToSignedData(signerInfo, *token)
		}
		if err != nil {
			return nil, fmt.Errorf("Timestamping failed: %s", err)
		}
	}
	return asn1.Marshal(*psd)
}