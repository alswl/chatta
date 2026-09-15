//go:build darwin || linux

// Package irc is the built-in IRC transport that replaces the external ii
// client: it dials the server, registers, and speaks the protocol directly
// in-process.
package irc
