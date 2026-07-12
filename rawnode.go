package whatsmeow

import waBinary "go.mau.fi/whatsmeow/binary"

// RawNodeEvent is delivered to Client.RawNodeHandler for every raw binary node
// sent and received, giving transport-level visibility equivalent to WA Web's
// (used by the WAM telemetry emitter to derive stanza-level analytics events).
type RawNodeEvent struct {
	// Node is the raw node. For received nodes it is the freshly decoded node
	// before normal processing; do not mutate it.
	Node *waBinary.Node
	// Outgoing is true for nodes this client sent, false for received nodes.
	Outgoing bool
	// Handled reports, for received nodes, whether whatsmeow has a handler for
	// the node's tag (or it is an ack/iq response). Always true for outgoing
	// nodes. A received node with Handled == false is effectively an unhandled
	// stanza.
	Handled bool
}
