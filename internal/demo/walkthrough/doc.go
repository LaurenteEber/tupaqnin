// Package walkthrough runs the jury's case 5 walkthrough end to end
// (backlog-r1 1.8, plan §1, H-1; pantallas.md «Recorrido del jurado»,
// steps 1 to 6) through the HTTP API, with the real cycle: with the fake
// provider and the recorded answers by default, and against Meta with
// make demo-check (-tags=meta), which records the answers again. It also
// runs the rushed profile's ten walkthroughs (backlog-r2 2.10), each with
// hand-made outputs and the brake of the table «Frenos esperados», and
// the slice-2 stretch of case 5 against Meta in make demo-check. Slice 3
// adds the round of case 5 and the citation that does not exist, and
// completes the scripts 4B and 6B (backlog-r3 3.7); make demo-check runs
// that round against Meta and the real OpenAlex. Slice 4 adds the full scripts of
// cases 7 and 9 and the stretches of 3B, 6B, 8B and 10B that the earlier walkthroughs
// cut (backlog-r4 4.6), each asserting the brake of its row in the table «Tabla de
// frenos de la 4.6»; make demo-check runs a stretch of case 9 against Meta.
package walkthrough
