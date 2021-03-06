// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// test2 should fail log check because HalfType2.Method1
// needs a logging construct and does not have one.
// It is tricky, because HalfType2 itself does not
// implement the interface, but HalfType3 which embeds
// HalfType2 does, and doing so, will make
// HalfType2.Method1() part of the implementation.
package test2

type Type1 struct{}

func (Type1) Method1() {
	//nologcall
}
func (Type1) Method2(int) {
	//nologcall
}

type HalfType2 struct{}

func (HalfType2) Method1() {
}

type HalfType3 struct {
	HalfType2
}

func (HalfType3) Method2(int) {
	//nologcall
}
