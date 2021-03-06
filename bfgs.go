// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"github.com/gonum/floats"
	"github.com/gonum/matrix/mat64"
)

// BFGS implements the Method interface to perform the Broyden–Fletcher–Goldfarb–Shanno
// optimization method with the given linesearch method. If LinesearchMethod is nil,
// it will be set to a reasonable default.
//
// BFGS is a quasi-Newton method that performs successive rank-one updates to
// an estimate of the inverse-Hessian of the function. It exhibits super-linear
// convergence when in proximity to a local minimum. It has memory cost that is
// O(n^2) relative to the input dimension.
type BFGS struct {
	LinesearchMethod LinesearchMethod

	linesearch *Linesearch

	x    []float64 // location of the last major iteration
	grad []float64 // gradient at the last major iteration
	dim  int

	// Temporary memory
	y       []float64
	yVec    *mat64.Vector
	s       []float64
	tmpData []float64
	tmpVec  *mat64.Vector

	invHess *mat64.SymDense

	first bool // Is it the first iteration (used to set the scale of the initial hessian)
}

// NOTE: This method exists so that it's easier to use a bfgs algorithm because
// it implements Method

func (b *BFGS) Init(loc *Location, p *ProblemInfo, xNext []float64) (EvaluationType, IterationType, error) {
	if b.LinesearchMethod == nil {
		b.LinesearchMethod = &Bisection{}
	}
	if b.linesearch == nil {
		b.linesearch = &Linesearch{}
	}
	b.linesearch.Method = b.LinesearchMethod
	b.linesearch.NextDirectioner = b

	return b.linesearch.Init(loc, p, xNext)
}

func (b *BFGS) Iterate(loc *Location, xNext []float64) (EvaluationType, IterationType, error) {
	return b.linesearch.Iterate(loc, xNext)
}

func (b *BFGS) InitDirection(loc *Location, dir []float64) (stepSize float64) {
	dim := len(loc.X)
	b.dim = dim

	b.x = resize(b.x, dim)
	copy(b.x, loc.X)
	b.grad = resize(b.grad, dim)
	copy(b.grad, loc.Gradient)

	b.y = resize(b.y, dim)
	b.s = resize(b.s, dim)
	b.tmpData = resize(b.tmpData, dim)
	b.yVec = mat64.NewVector(dim, b.y)
	b.tmpVec = mat64.NewVector(dim, b.tmpData)

	if b.invHess == nil || cap(b.invHess.RawSymmetric().Data) < dim*dim {
		b.invHess = mat64.NewSymDense(dim, nil)
	} else {
		b.invHess = mat64.NewSymDense(dim, b.invHess.RawSymmetric().Data[:dim*dim])
	}

	// The values of the hessian are initialized in the first call to NextDirection

	// initial direcion is just negative of gradient because the hessian is 1
	copy(dir, loc.Gradient)
	floats.Scale(-1, dir)

	b.first = true

	return 1 / floats.Norm(dir, 2)
}

func (b *BFGS) NextDirection(loc *Location, dir []float64) (stepSize float64) {
	if len(loc.X) != b.dim {
		panic("bfgs: unexpected size mismatch")
	}
	if len(loc.Gradient) != b.dim {
		panic("bfgs: unexpected size mismatch")
	}
	if len(dir) != b.dim {
		panic("bfgs: unexpected size mismatch")
	}

	// Compute the gradient difference in the last step
	// y = g_{k+1} - g_{k}
	floats.SubTo(b.y, loc.Gradient, b.grad)

	// Compute the step difference
	// s = x_{k+1} - x_{k}
	floats.SubTo(b.s, loc.X, b.x)

	sDotY := floats.Dot(b.s, b.y)
	sDotYSquared := sDotY * sDotY

	if b.first {
		// Rescale the initial hessian.
		// From: Numerical optimization, Nocedal and Wright, Page 143, Eq. 6.20 (second edition).
		yDotY := floats.Dot(b.y, b.y)
		scale := sDotY / yDotY
		for i := 0; i < len(loc.X); i++ {
			for j := 0; j < len(loc.X); j++ {
				if i == j {
					b.invHess.SetSym(i, i, scale)
				} else {
					b.invHess.SetSym(i, j, 0)
				}
			}
		}
		b.first = false
	}

	// Compute the update rule
	//     B_{k+1}^-1
	// First term is just the existing inverse hessian
	// Second term is
	//     (sk^T yk + yk^T B_k^-1 yk)(s_k sk_^T) / (sk^T yk)^2
	// Third term is
	//     B_k ^-1 y_k sk^T + s_k y_k^T B_k-1
	//
	// y_k^T B_k^-1 y_k is a scalar, and the third term is a rank-two update
	// where B_k^-1 y_k is one vector and s_k is the other. Compute the update
	// values then actually perform the rank updates.
	yBy := mat64.Inner(b.yVec, b.invHess, b.yVec)
	firstTermConst := (sDotY + yBy) / (sDotYSquared)
	b.tmpVec.MulVec(b.invHess, false, b.yVec)

	b.invHess.RankTwo(b.invHess, -1/sDotY, b.tmpData, b.s)
	b.invHess.SymRankOne(b.invHess, firstTermConst, b.s)

	// update the bfgs stored data to the new iteration
	copy(b.x, loc.X)
	copy(b.grad, loc.Gradient)

	// Compute the new search direction
	dirmat := mat64.NewDense(b.dim, 1, dir)
	gradmat := mat64.NewDense(b.dim, 1, loc.Gradient)

	dirmat.Mul(b.invHess, gradmat) // new direction stored in place
	floats.Scale(-1, dir)
	return 1
}

func (*BFGS) Needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{true, false}
}
