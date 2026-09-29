package relay

import (
	"encoding/json"
	"time"
)

// The frame types. Anything else on the socket is ignored, so a newer peer
// does not cost an older one its connection.
const (
	// TypeHello is the first frame a client sends and the only one carrying
	// the relay key.
	TypeHello = "hello"
	// TypeAnnounce is sent by the relay only: who else is on this key.
	TypeAnnounce = "announce"
	// TypePresence is sent by the relay only: a sibling's connection went away.
	TypePresence = "presence"
	// TypeProxyRequest wraps one call to a sibling.
	TypeProxyRequest = "proxy-request"
	// TypeProxyResponse is that call's answer, matched by request id.
	TypeProxyResponse = "proxy-response"
)

// Envelope is every frame on the socket: a type plus that type's payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Hello authenticates and introduces a connection in one frame. The key
// rides the first frame rather than the URL because reverse proxies log URLs.
type Hello struct {
	Key      string   `json:"key"`
	Announce Announce `json:"announce"`
}

// Announce is an instance introducing itself on the wire. InstanceID is the
// one field the relay routes on and so the only one in the clear; the
// identity travels in Sealed, bound to InstanceID.
type Announce struct {
	InstanceID string `json:"instanceId"`
	Sealed     []byte `json:"sealed,omitempty"`
}

// Identity is the part of an announce the relay never reads.
type Identity struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// Sibling is another group member as the relay makes it visible, with its
// identity opened.
type Sibling struct {
	InstanceID string
	Identity
}

// announceAAD binds an announce's seal to its instance id. The label differs
// from the call and result labels, so no blob opens as another frame type.
func announceAAD(instanceID string) string {
	return "announce\x00" + instanceID
}

// SealIdentity seals the identity half of an announce.
func SealIdentity(key []byte, instanceID string, id Identity) ([]byte, error) {
	plain, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	return seal(key, announceAAD(instanceID), plain)
}

// OpenIdentity reverses SealIdentity.
func OpenIdentity(key []byte, instanceID string, sealed []byte) (Identity, error) {
	plain, err := open(key, announceAAD(instanceID), sealed)
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := json.Unmarshal(plain, &id); err != nil {
		return Identity{}, ErrSealed
	}
	return id, nil
}

// Presence reports that a sibling went offline. An arrival is an Announce.
type Presence struct {
	InstanceID string `json:"instanceId"`
	Online     bool   `json:"online"`
}

// ProxyRequest is the wire form of one call to a sibling: the two fields the
// relay routes on and a sealed ProxyCall it cannot read. The direct transport
// on the local network sends the same shape.
type ProxyRequest struct {
	RequestID string `json:"requestId"`
	Target    string `json:"target"`
	Sealed    []byte `json:"sealed,omitempty"`
}

// ProxyCall is what a ProxyRequest asks once opened. It has no header map, so
// a sender cannot set a cookie or a forwarded address on the replayed request.
type ProxyCall struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   []byte `json:"body,omitempty"`
	// ID and Sent are the request id and the Unix time the call was sealed
	// at. They travel inside the seal, where a relay cannot change them, so
	// the receiver can refuse a frame it has run before or one that is old.
	ID   string `json:"id"`
	Sent int64  `json:"sent"`
	// Sender is the instance id of the caller, set by the same key that seals
	// the rest of the call. Direct and relay calls both carry it here, so a
	// handler names who asked for something regardless of which transport
	// carried the call. Like every other claim under the group key, it is
	// claimed rather than proven: any member can seal a call under this key.
	Sender string `json:"sender,omitempty"`
}

// ProxyResponse is the wire form of the answer to one ProxyRequest.
type ProxyResponse struct {
	RequestID string `json:"requestId"`
	Sealed    []byte `json:"sealed,omitempty"`
	// Error is set only when the relay answers instead of the target. It
	// stays in the clear because the relay has no key, and a caller never
	// takes an unsealed response for a result.
	Error string `json:"error,omitempty"`
}

// ProxyResult is what a ProxyResponse answers once opened.
type ProxyResult struct {
	Status int    `json:"status"`
	Body   []byte `json:"body,omitempty"`
}

func requestAAD(requestID, target string) string {
	return "proxy-request\x00" + requestID + "\x00" + target
}

func responseAAD(requestID string) string {
	return "proxy-response\x00" + requestID
}

// SealCall seals one call, bound to its request id and target, so a relay
// that redirects it to another instance produces a frame that does not open.
// It stamps the call with requestID and, unless Sent is set, the current time.
func SealCall(key []byte, requestID, target string, call ProxyCall) ([]byte, error) {
	call.ID = requestID
	if call.Sent == 0 {
		call.Sent = time.Now().Unix()
	}
	plain, err := json.Marshal(call)
	if err != nil {
		return nil, err
	}
	return seal(key, requestAAD(requestID, target), plain)
}

// OpenCall reverses SealCall.
func OpenCall(key []byte, requestID, target string, sealed []byte) (ProxyCall, error) {
	plain, err := open(key, requestAAD(requestID, target), sealed)
	if err != nil {
		return ProxyCall{}, err
	}
	var call ProxyCall
	if err := json.Unmarshal(plain, &call); err != nil || call.ID != requestID {
		return ProxyCall{}, ErrSealed
	}
	return call, nil
}

// SealResult seals one answer, bound to its request id.
func SealResult(key []byte, requestID string, result ProxyResult) ([]byte, error) {
	plain, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return seal(key, responseAAD(requestID), plain)
}

// OpenResult reverses SealResult.
func OpenResult(key []byte, requestID string, sealed []byte) (ProxyResult, error) {
	plain, err := open(key, responseAAD(requestID), sealed)
	if err != nil {
		return ProxyResult{}, err
	}
	var res ProxyResult
	if err := json.Unmarshal(plain, &res); err != nil {
		return ProxyResult{}, ErrSealed
	}
	return res, nil
}

// Encode marshals one frame ready to be written to the socket.
func Encode(typ string, data any) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: typ, Data: payload})
}

// Decode reads a frame's envelope without its payload, so the relay can route
// on the type and forward the original bytes.
func Decode(frame []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// Into unmarshals the payload of a decoded frame.
func (e Envelope) Into(v any) error {
	return json.Unmarshal(e.Data, v)
}
