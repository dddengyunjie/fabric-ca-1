/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ocsp"

	"github.com/hyperledger/fabric-ca/api"
	"github.com/hyperledger/fabric-ca/util"
	"github.com/stretchr/testify/assert"
)

func TestGetAllAffiliations(t *testing.T) {
	os.RemoveAll(rootDir)
	defer os.RemoveAll(rootDir)

	var err error

	srv := TestGetRootServer(t)
	srv.RegisterBootstrapUser("admin2", "admin2pw", "org2")
	err = srv.Start()
	util.FatalError(t, err, "Failed to start server")
	defer srv.Stop()

	client := getTestClient(7075)
	resp, err := client.Enroll(&api.EnrollmentRequest{
		Name:   "admin",
		Secret: "adminpw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin := resp.Identity

	resp, err = client.Enroll(&api.EnrollmentRequest{
		Name:   "admin2",
		Secret: "admin2pw",
	})

	admin2 := resp.Identity

	result, err := captureOutput(admin.GetAllAffiliations, "", AffiliationDecoder)
	assert.NoError(t, err, "Failed to get all affiliations")

	affiliations := []AffiliationRecord{}
	err = srv.CA.db.Select(&affiliations, srv.CA.db.Rebind("SELECT * FROM affiliations"))
	if err != nil {
		t.Error("Failed to get all affiliations in database")
	}

	for _, aff := range affiliations {
		if !strings.Contains(result, aff.Name) {
			t.Error("Failed to get all appropriate affiliations")
		}
	}

	// admin2's affilations is "org2"
	result, err = captureOutput(admin2.GetAllAffiliations, "", AffiliationDecoder)
	assert.NoError(t, err, "Failed to get all affiliations for admin2")

	if !strings.Contains(result, "org2") {
		t.Error("Incorrect affiliation received")
	}

	notAffMgr, err := admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name: "notAffMgr",
	})
	util.FatalError(t, err, "Failed to register a user that is not affiliation manager")

	err = notAffMgr.GetAllAffiliations("", AffiliationDecoder)
	if assert.Error(t, err, "Should have failed, as the caller does not have the attribute 'hf.AffiliationMgr'") {
		assert.Contains(t, err.Error(), "User does not have attribute 'hf.AffiliationMgr'")
	}

}

func TestGetAffiliation(t *testing.T) {
	os.RemoveAll(rootDir)
	defer os.RemoveAll(rootDir)

	var err error

	srv := TestGetRootServer(t)
	srv.RegisterBootstrapUser("admin2", "admin2pw", "org2")
	err = srv.Start()
	util.FatalError(t, err, "Failed to start server")
	defer srv.Stop()

	client := getTestClient(7075)
	resp, err := client.Enroll(&api.EnrollmentRequest{
		Name:   "admin",
		Secret: "adminpw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin := resp.Identity

	resp, err = client.Enroll(&api.EnrollmentRequest{
		Name:   "admin2",
		Secret: "admin2pw",
	})

	admin2 := resp.Identity

	getAffResp, err := admin.GetAffiliation("org2.dept1", "")
	assert.NoError(t, err, "Failed to get requested affiliations")
	assert.Equal(t, "org2.dept1", getAffResp.Info.Name)

	getAffResp, err = admin2.GetAffiliation("org1", "")
	assert.Error(t, err, "Should have failed, caller not authorized to get affiliation")

	getAffResp, err = admin2.GetAffiliation("org2.dept2", "")
	assert.Error(t, err, "Should have returned an error, requested affiliation does not exist")

	getAffResp, err = admin2.GetAffiliation("org2.dept1", "")
	assert.NoError(t, err, "Failed to get requested affiliation")

	notAffMgr, err := admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name: "notAffMgr",
	})
	util.FatalError(t, err, "Failed to register a user that is not affiliation manager")

	_, err = notAffMgr.GetAffiliation("org2", "")
	assert.Error(t, err, "Should have failed, as the caller does not have the attribute 'hf.AffiliationMgr'")
}

func TestDynamicAddAffiliation(t *testing.T) {
	os.RemoveAll(rootDir)
	defer os.RemoveAll(rootDir)

	var err error

	srv := TestGetRootServer(t)
	srv.RegisterBootstrapUser("admin2", "admin2pw", "org2")
	err = srv.Start()
	util.FatalError(t, err, "Failed to start server")
	defer srv.Stop()

	client := getTestClient(7075)
	resp, err := client.Enroll(&api.EnrollmentRequest{
		Name:   "admin",
		Secret: "adminpw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin := resp.Identity

	// Register an admin with "hf.AffiliationMgr" role
	notAffMgr, err := admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name: "notAffMgr",
		Attributes: []api.Attribute{
			api.Attribute{
				Name:  "hf.AffiliationMgr",
				Value: "false",
			},
		},
	})

	resp, err = client.Enroll(&api.EnrollmentRequest{
		Name:   "admin2",
		Secret: "admin2pw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin2 := resp.Identity

	addAffReq := &api.AddAffiliationRequest{}
	addAffReq.Info.Name = "org3"

	addAffResp, err := notAffMgr.AddAffiliation(addAffReq)
	assert.Error(t, err, "Should have failed, caller does not have 'hf.AffiliationMgr' attribute")

	addAffResp, err = admin2.AddAffiliation(addAffReq)
	assert.Error(t, err, "Should have failed affiliation, caller's affilation is 'org2'. Caller can't add affiliation 'org3'")

	addAffResp, err = admin.AddAffiliation(addAffReq)
	util.FatalError(t, err, "Failed to add affiliation 'org3'")
	assert.Equal(t, "org3", addAffResp.Info.Name)

	addAffResp, err = admin.AddAffiliation(addAffReq)
	assert.Error(t, err, "Should have failed affiliation 'org3' already exists")

	addAffReq.Info.Name = "org3.dept1"
	addAffResp, err = admin.AddAffiliation(addAffReq)
	assert.NoError(t, err, "Failed to affiliation")

	registry := srv.registry
	_, err = registry.GetAffiliation("org3.dept1")
	assert.NoError(t, err, "Failed to add affiliation correctly")

	addAffReq.Info.Name = "org4.dept1.team2"
	addAffResp, err = admin.AddAffiliation(addAffReq)
	assert.Error(t, err, "Should have failed, parent affiliation does not exist. Force option is required")

	addAffReq.Force = true
	addAffResp, err = admin.AddAffiliation(addAffReq)
	assert.NoError(t, err, "Failed to add multiple affiliations with force option")

	_, err = registry.GetAffiliation("org4.dept1.team2")
	assert.NoError(t, err, "Failed to add affiliation correctly")

	_, err = registry.GetAffiliation("org4.dept1")
	assert.NoError(t, err, "Failed to add affiliation correctly")
	assert.Equal(t, "org4.dept1.team2", addAffResp.Info.Name)
}

func TestDynamicRemoveAffiliation(t *testing.T) {
	os.RemoveAll(rootDir)
	defer os.RemoveAll(rootDir)

	var err error

	srv := TestGetRootServer(t)
	srv.RegisterBootstrapUser("admin2", "admin2pw", "org2")
	err = srv.Start()
	util.FatalError(t, err, "Failed to start server")
	defer srv.Stop()

	client := getTestClient(7075)
	resp, err := client.Enroll(&api.EnrollmentRequest{
		Name:   "admin",
		Secret: "adminpw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin := resp.Identity

	resp, err = client.Enroll(&api.EnrollmentRequest{
		Name:   "admin2",
		Secret: "admin2pw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin2'")

	admin2 := resp.Identity

	_, err = admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name:        "testuser1",
		Affiliation: "org2",
	})
	assert.NoError(t, err, "Failed to register and enroll 'testuser1'")

	notRegistrar, err := admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name: "notregistrar",
		Attributes: []api.Attribute{
			api.Attribute{
				Name:  "hf.AffiliationMgr",
				Value: "true",
			},
		},
	})
	assert.NoError(t, err, "Failed to register and enroll 'notregistrar'")

	registry := srv.CA.registry
	_, err = registry.GetUser("testuser1", nil)
	assert.NoError(t, err, "User should exist")

	certdbregistry := srv.CA.certDBAccessor
	certs, err := certdbregistry.GetCertificatesByID("testuser1")
	if len(certs) != 1 {
		t.Error("Failed to correctly enroll identity")
	}

	_, err = admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name:        "testuser2",
		Affiliation: "org2",
	})
	assert.NoError(t, err, "Failed to register and enroll 'testuser1'")

	_, err = registry.GetUser("testuser2", nil)
	assert.NoError(t, err, "User should exist")

	certs, err = certdbregistry.GetCertificatesByID("testuser2")
	if len(certs) != 1 {
		t.Error("Failed to correctly enroll identity")
	}

	removeAffReq := &api.RemoveAffiliationRequest{
		Name: "org2",
	}

	_, err = admin.RemoveAffiliation(removeAffReq)
	assert.Error(t, err, "Should have failed, affiliation removal not allowed")

	srv.CA.Config.Cfg.Affiliations.AllowRemove = true

	_, err = admin2.RemoveAffiliation(removeAffReq)
	assert.Error(t, err, "Should have failed, can't remove affiliation as the same level as caller")

	_, err = admin.RemoveAffiliation(removeAffReq)
	assert.Error(t, err, "Should have failed, there is an identity associated with affiliation. Need to use force option")

	removeAffReq.Force = true
	_, err = admin.RemoveAffiliation(removeAffReq)
	assert.Error(t, err, "Should have failed, there is an identity associated with affiliation but identity removal is not allowed")

	srv.CA.Config.Cfg.Identities.AllowRemove = true

	_, err = notRegistrar.RemoveAffiliation(removeAffReq)
	if assert.Error(t, err, "Should have failed, there is an identity associated with affiliation but caller is not a registrar") {
		assert.Contains(t, err.Error(), "Authorization failure")
	}

	removeResp, err := admin.RemoveAffiliation(removeAffReq)
	assert.NoError(t, err, "Failed to remove affiliation")

	_, err = registry.GetUser("testuser1", nil)
	assert.Error(t, err, "User should not exist")

	_, err = registry.GetUser("testuser2", nil)
	assert.Error(t, err, "User should not exist")

	certs, err = certdbregistry.GetCertificatesByID("testuser1")
	if certs[0].Status != "revoked" && certs[0].Reason != ocsp.AffiliationChanged {
		t.Error("Failed to correctly revoke certificate for an identity whose affiliation was removed")
	}

	certs, err = certdbregistry.GetCertificatesByID("testuser2")
	if certs[0].Status != "revoked" || certs[0].Reason != ocsp.AffiliationChanged {
		t.Error("Failed to correctly revoke certificate for an identity whose affiliation was removed")
	}

	assert.Equal(t, "org2.dept1", removeResp.Affiliations[0].Name)
	assert.Equal(t, "org2", removeResp.Affiliations[1].Name)
	assert.Equal(t, "admin2", removeResp.Identities[0].ID)

	_, err = admin.RemoveAffiliation(removeAffReq)
	assert.Error(t, err, "Should have failed, trying to remove an affiliation that does not exist")
}

func TestDynamicModifyAffiliation(t *testing.T) {
	os.RemoveAll(rootDir)
	defer os.RemoveAll(rootDir)

	var err error

	srv := TestGetRootServer(t)
	srv.RegisterBootstrapUser("admin2", "admin2pw", "hyperledger")
	err = srv.Start()
	util.FatalError(t, err, "Failed to start server")
	defer srv.Stop()

	client := getTestClient(7075)
	resp, err := client.Enroll(&api.EnrollmentRequest{
		Name:   "admin",
		Secret: "adminpw",
	})
	util.FatalError(t, err, "Failed to enroll user 'admin'")

	admin := resp.Identity

	notRegistrar, err := admin.RegisterAndEnroll(&api.RegistrationRequest{
		Name:        "testuser1",
		Affiliation: "org2",
		Attributes: []api.Attribute{
			api.Attribute{
				Name:  "hf.AffiliationMgr",
				Value: "true",
			},
		},
	})

	modifyAffReq := &api.ModifyAffiliationRequest{
		Name: "org2",
	}
	modifyAffReq.Info.Name = "org3"

	_, err = admin.ModifyAffiliation(modifyAffReq)
	assert.Error(t, err, "Should have failed, there is an identity associated with affiliation. Need to use force option")

	modifyAffReq.Force = true
	modifyResp, err := notRegistrar.ModifyAffiliation(modifyAffReq)
	if assert.Error(t, err, "Should have failed to modify affiliation, identities are affected but caller is not a registrar") {
		assert.Contains(t, err.Error(), "Authorization failure")
	}

	modifyResp, err = admin.ModifyAffiliation(modifyAffReq)
	assert.NoError(t, err, "Failed to modify affiliation")

	registry := srv.registry
	_, err = registry.GetAffiliation("org3")
	assert.NoError(t, err, "Failed to modify affiliation to 'org3'")

	user, err := registry.GetUser("testuser1", nil)
	util.FatalError(t, err, "Failed to get user")

	userAff := GetUserAffiliation(user)
	assert.Equal(t, "org3", userAff)

	assert.Equal(t, "org2.dept1", modifyResp.Affiliations[0].Name)
	assert.Equal(t, "org2", modifyResp.Affiliations[1].Name)
	assert.Equal(t, "testuser1", modifyResp.Identities[0].ID)
}