// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"github.com/gonum/floats"
	"github.com/gonum/matrix/mat64"
)

const maxNewtonModifications = 20

// Newton implements a modified Newton's method for Hessian-based unconstrained
// minimization. It applies regularization when the Hessian is not positive
// definite, and it can converge to a local minimum from any starting point.
//
// Newton iteratively forms a quadratic model to the objective function f and
// tries to minimize this approximate model. It generates a sequence of
// locations x_k by means of
//  solve H_k d_k = -∇f_k for d_k,
//  x_{k+1} = x_k + α_k d_k,
// where H_k is the Hessian matrix of f at x_k and α_k is a step size found by
// a line search.
//
// Away from a minimizer H_k may not be positive definite and d_k may not be a
// descent direction. Newton implements a Hessian modification strategy that
// adds successively larger multiples of identity to H_k until it becomes
// positive definite. Note that the repeated trial factorization of the
// modified Hessian involved in this process can be computationally expensive.
//
// If the Hessian matrix cannot be formed explicitly or if the computational
// cost of its factorization is prohibitive, BFGS or L-BFGS quasi-Newton method
// can be used instead.
type Newton struct {
	// LinesearchMethod is a method used for selecting suitable steps along the
	// descent direction d. Steps should satisfy at least one of the Wolfe,
	// Goldstein or Armijo conditions. If LinesearchMethod == nil, an
	// appropriate default is chosen.
	LinesearchMethod LinesearchMethod
	// Increase is the factor by which a scalar tau is successively increased
	// so that (H + tau*I) is positive definite. Larger values reduce the
	// number of trial Hessian factorizations, but also reduce the second-order
	// information in H.
	// Increase must be greater than 1. If Increase is 0, it is defaulted to 5.
	Increase float64

	linesearch *Linesearch

	hess *mat64.SymDense // Storage for a copy of the Hessian matrix.
	chol *mat64.TriDense // Storage for the Cholesky factorization.
	tau  float64
}

func (n *Newton) Init(loc *Location, p *ProblemInfo, xNext []float64) (EvaluationType, IterationType, error) {
	if n.Increase == 0 {
		n.Increase = 5
	}
	if n.Increase <= 1 {
		panic("optimize: Newton.Increase must be greater than 1")
	}
	if n.LinesearchMethod == nil {
		n.LinesearchMethod = &Bisection{}
	}
	if n.linesearch == nil {
		n.linesearch = &Linesearch{}
	}
	n.linesearch.Method = n.LinesearchMethod
	n.linesearch.NextDirectioner = n

	return n.linesearch.Init(loc, p, xNext)
}

func (n *Newton) Iterate(loc *Location, xNext []float64) (EvaluationType, IterationType, error) {
	return n.linesearch.Iterate(loc, xNext)
}

func (n *Newton) InitDirection(loc *Location, dir []float64) (stepSize float64) {
	dim := len(loc.X)
	n.chol = resizeTriDense(n.chol, dim)
	n.hess = resizeSymDense(n.hess, dim)
	n.tau = 0
	return n.NextDirection(loc, dir)
}

func (n *Newton) NextDirection(loc *Location, dir []float64) (stepSize float64) {
	// This method implements Algorithm 3.3 (Cholesky with Added Multiple of
	// the Identity) from Nocedal, Wright (2006), 2nd edition.

	dim := len(loc.X)
	n.hess.CopySym(loc.Hessian)

	// Find the smallest diagonal entry of the Hesssian.
	minA := n.hess.At(0, 0)
	for i := 1; i < dim; i++ {
		a := n.hess.At(i, i)
		if a < minA {
			minA = a
		}
	}
	// If the smallest diagonal entry is positive, the Hessian may be positive
	// definite, and so first attempt to apply the Cholesky factorization to
	// the un-modified Hessian. If the smallest entry is negative, use the
	// final tau from the last iteration if regularization was needed,
	// otherwise guess an appropriate value for tau.
	if minA > 0 {
		n.tau = 0
	} else if n.tau == 0 {
		n.tau = -minA + 0.001
	}

	for k := 0; k < maxNewtonModifications; k++ {
		if n.tau != 0 {
			// Add a multiple of identity to the Hessian.
			for i := 0; i < dim; i++ {
				n.hess.SetSym(i, i, loc.Hessian.At(i, i)+n.tau)
			}
		}
		// Try to apply the Cholesky factorization.
		pd := n.chol.Cholesky(n.hess, true)
		if pd {
			d := mat64.NewVector(dim, dir)
			// Store the solution in d's backing array, dir.
			d.SolveCholeskyVec(n.chol, mat64.NewVector(dim, loc.Gradient))
			floats.Scale(-1, dir)
			return 1
		}
		// Modified Hessian is not PD, so increase tau.
		n.tau = math.Max(n.Increase*n.tau, 0.001)
	}

	// Hessian modification failed to get a PD matrix. Return the negative
	// gradient as the descent direction.
	copy(dir, loc.Gradient)
	floats.Scale(-1, dir)
	return 1
}

func (n *Newton) Needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{true, true}
}
