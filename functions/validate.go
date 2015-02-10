// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package functions provides objective functions for testing optimization
// algorithms.
//
// We encourage outside contributions of additional test functions that exhibit
// properties not already covered in the testing suite or that have
// significance due to prior use as benchmark cases.
package functions

import (
	"math"
	"testing"

	"github.com/gonum/diff/fd"
	"github.com/gonum/floats"
)

// function represents an objective function.
type function interface {
	F(x []float64) float64
}

type gradient interface {
	Df(x, grad []float64)
}

type functionGradient interface {
	FDf(x, grad []float64) float64
}

// minimumer is an objective function that can also provide information about
// its minima.
type minimumer interface {
	function

	// Minima returns _known_ minima of the function.
	Minima() []Minimum
}

// Minimum represents information about an optimal location of a function.
type Minimum struct {
	// X is the location of the minimum. X may not be nil.
	X []float64
	// F is the value of the objective function at X.
	F float64
	// Global indicates if the location is a global minimum.
	Global bool
}

type funcTest struct {
	X []float64

	// F is the expected function value at X.
	F float64
	// Gradient is the expected gradient at X. If nil, it is not evaluated.
	Gradient []float64
}

// TODO(vladimir-ch): Decide and implement an exported testing function:
// func Test(f Function, ??? ) ??? {
// }

const (
	defaultTol       = 1e-12
	defaultGradTol   = 1e-9
	defaultFDGradTol = 1e-5
)

// testFunction checks that the function can evaluate itself (and its gradient)
// correctly.
func testFunction(f function, ftests []funcTest, t *testing.T) {
	// Make a copy of tests because we may append to the slice.
	tests := make([]funcTest, len(ftests))
	copy(tests, ftests)

	// Get information about the function.
	fMinima, isMinimumer := f.(minimumer)
	fGradient, isGradient := f.(gradient)
	fFunctionGradient, isFunctionGradient := f.(functionGradient)

	// If the function is a Minimumer, append its minima to the tests.
	if isMinimumer {
		for _, minimum := range fMinima.Minima() {
			// Allocate gradient only if the function can evaluate it.
			var grad []float64
			if isGradient || isFunctionGradient {
				grad = make([]float64, len(minimum.X))
				for i := range grad {
					grad[i] = 0
				}
			}
			tests = append(tests, funcTest{
				X:        minimum.X,
				F:        minimum.F,
				Gradient: grad,
			})
		}
	}

	for i, test := range tests {
		F := f.F(test.X)

		// Check that the function value is as expected.
		if math.Abs(F-test.F) > defaultTol {
			t.Errorf("Test #%d: function value given by F() is incorrect. Want: %v, Got: %v",
				i, test.F, F)
		}

		if test.Gradient == nil {
			continue
		}

		// Evaluate the finite difference gradient.
		fdGrad := fd.Gradient(nil, f.F, test.X, nil)

		// Check that the finite difference and expected gradients match.
		if !floats.EqualApprox(fdGrad, test.Gradient, defaultFDGradTol) {
			dist := floats.Distance(fdGrad, test.Gradient, math.Inf(1))
			t.Errorf("Test #%d: numerical and expected gradients do not match. |fdGrad - WantGrad|_∞ = %v",
				i, dist)
		}

		// If the function is a Gradient, check that it computes the gradient correctly.
		if isGradient {
			grad := make([]float64, len(test.Gradient))
			fGradient.Df(test.X, grad)

			if !floats.EqualApprox(grad, test.Gradient, defaultGradTol) {
				dist := floats.Distance(grad, test.Gradient, math.Inf(1))
				t.Errorf("Test #%d: gradient given by Df() is incorrect. |grad - WantGrad|_∞ = %v",
					i, dist)
			}
		}

		// If the function is a FunctionGradient, check that it computes its
		// value and the gradient correctly.
		if isFunctionGradient {
			grad := make([]float64, len(test.Gradient))
			F := fFunctionGradient.FDf(test.X, grad)

			if math.Abs(F-test.F) > defaultTol {
				t.Errorf("Test #%d: function value given by FDf() is incorrect. Want: %v, Got: %v",
					i, test.F, F)
			}

			if !floats.EqualApprox(grad, test.Gradient, defaultGradTol) {
				dist := floats.Distance(grad, test.Gradient, math.Inf(1))
				t.Errorf("Test #%d: gradient given by FDf() is incorrect. |grad - WantGrad|_∞ = %v",
					i, dist)
			}
		}
	}
}