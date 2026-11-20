# Working in `ir`

Notes for changing `ir`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers damage, comparing styles, and wrappers that hide optional interfaces.

## Things that will bite

**Damage is per drawing call, and reports `ok == false` rather than guessing.**
`ir.Damage` compares two recordings call for call; a different call count, a
different kind, or a moved transform means the chart's *structure* changed and
the answer is a full repaint. Do not make it clever about realigning: a list of
rectangles that describes half a frame is worse than repainting the frame.

**`ir.Partial.Damage(nil)` means the whole frame, and an empty list never
arrives.** A frame identical to the last one is not painted at all rather than
painted with no damage, which is why the nil case is unambiguous.

**A Stroke cannot be compared with `==`.** It holds a dash slice. `ir` carries
`Stroke.same`, `Fill.same` and `MarkerStyle.same` for this; reach for those
rather than adding a comparison that compiles today because the field you would
have missed happens not to be there yet.

**A backend wrapper hides the optional interfaces of what it wraps.** `Damage`,
`Resize` and `Describe` are all reached by type assertion, so a wrapper that
does not declare them silently drops them. `ir.Recorder` and `interact`'s probe
both forward `Describe`; `render.Draw` keeps the *unwrapped* backend for the
semantics question, because it wraps the backend to install a typesetter. A
`<title>` that quietly disappears from interactive charts only is what this
costs when it is forgotten, and there is a test.
