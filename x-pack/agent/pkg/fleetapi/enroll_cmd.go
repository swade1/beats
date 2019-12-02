// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

// EnrollType is the type of enrollment to do with the agent.
type EnrollType string

const (
	// PermanentEnroll is default enrollment type, by default an Agent is permanently enroll to Agent.
	PermanentEnroll = EnrollType("PERMANENT")
)

var mapEnrollType = map[string]EnrollType{
	"PERMANENT": PermanentEnroll,
}

var reverseMapEnrollType = make(map[EnrollType]string)

func init() {
	for k, v := range mapEnrollType {
		reverseMapEnrollType[v] = k
	}
}

// UnmarshalJSON unmarshal an enrollment type.
func (p *EnrollType) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) <= 2 {
		return errors.New("invalid enroll type received")
	}
	s = s[1 : len(s)-1]
	v, ok := mapEnrollType[s]
	if !ok {
		return fmt.Errorf("value of '%s' is an invalid enrollment type, supported type is 'PERMANENT'", s)
	}

	*p = v

	return nil
}

// MarshalJSON marshal an enrollType.
func (p EnrollType) MarshalJSON() ([]byte, error) {
	v, ok := reverseMapEnrollType[p]
	if !ok {
		return nil, errors.New("cannot serialize unknown type")
	}

	return json.Marshal(v)
}

// EnrollRequest is the data required to enroll the agent into Fleet.
//
// Example:
// POST /api/fleet/agents/enroll
// {
// 	"type": "PERMANENT",
//   "metadata": {
// 	  "local": { "os": "macos"},
// 	  "userProvided": { "region": "us-east"}
//   }
// }
type EnrollRequest struct {
	EnrollmentToken string     `json:"-"`
	Type            EnrollType `json:"type"`
	SharedID        string     `json:"sharedId,omitempty"`
	Metadata        Metadata   `json:"metadata"`
}

// Metadata is a all the metadata send or received from the agent.
type Metadata struct {
	Local        map[string]interface{} `json:"local"`
	UserProvided map[string]interface{} `json:"userProvided"`
}

// Validate validates the enrollment request before sending it to the API.
func (e *EnrollRequest) Validate() error {
	var err error

	if len(e.EnrollmentToken) == 0 {
		err = multierror.Append(err, errors.New("missing enrollment token"))
	}

	if len(e.Type) == 0 {
		err = multierror.Append(err, errors.New("missing enrollment type"))
	}

	return err
}

// EnrollResponse is the data received after enrolling an Agent into fleet.
//
// Example:
// {
//   "action": "created",
//   "success": true,
//   "item": {
//     "id": "a4937110-e53e-11e9-934f-47a8e38a522c",
//     "active": true,
//     "policy_id": "default",
//     "type": "PERMANENT",
//     "enrolled_at": "2019-10-02T18:01:22.337Z",
//     "user_provided_metadata": {},
//     "local_metadata": {},
//     "actions": [],
//     "access_token": "ACCESS_TOKEN"
//   }
// }
type EnrollResponse struct {
	Action  string             `json:"action"`
	Success bool               `json:"success"`
	Item    EnrollItemResponse `json:"item"`
}

// EnrollItemResponse item response.
type EnrollItemResponse struct {
	ID                   string                 `json:"id"`
	Active               bool                   `json:"active"`
	PolicyID             string                 `json:"policy_id"`
	Type                 EnrollType             `json:"type"`
	EnrolledAt           time.Time              `json:"enrolled_at"`
	UserProvidedMetadata map[string]interface{} `json:"user_provided_metadata"`
	LocalMetadata        map[string]interface{} `json:"local_metadata"`
	Actions              []interface{}          `json:"actions"`
	AccessToken          string                 `json:"access_token"`
}

// Validate validates the response send from the server.
func (e *EnrollResponse) Validate() error {
	var err error

	if len(e.Item.ID) == 0 {
		err = multierror.Append(err, errors.New("missing ID"))
	}

	if len(e.Item.Type) == 0 {
		err = multierror.Append(err, errors.New("missing enrollment type"))
	}

	if len(e.Item.AccessToken) == 0 {
		err = multierror.Append(err, errors.New("access token is missing"))
	}

	return err
}

// EnrollCmd is the command to be executed to enroll an agent into Fleet.
type EnrollCmd struct {
	client clienter
}

// Execute enroll the Agent in the Fleet.
func (e *EnrollCmd) Execute(r *EnrollRequest) (*EnrollResponse, error) {
	const p = "/api/fleet/agents/enroll"
	const key = "kbn-fleet-enrollment-token"

	if err := r.Validate(); err != nil {
		return nil, err
	}

	headers := map[string][]string{
		key: []string{r.EnrollmentToken},
	}

	b, err := json.Marshal(r)
	if err != nil {
		return nil, errors.Wrap(err, "fail to encode the enrollment request")
	}

	resp, err := e.client.Send("POST", p, nil, headers, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, extract(resp.Body)
	}

	enrollResponse := &EnrollResponse{}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(enrollResponse); err != nil {
		return nil, errors.Wrap(err, "fail to decode enrollment response")
	}

	if err := enrollResponse.Validate(); err != nil {
		return nil, err
	}

	return enrollResponse, nil
}

// NewEnrollCmd creates a new EnrollCmd.
func NewEnrollCmd(client clienter) *EnrollCmd {
	return &EnrollCmd{client: client}
}