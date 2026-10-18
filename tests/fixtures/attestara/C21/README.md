# C21 viewer fixtures

Repository-authored synthetic fixtures under Apache-2.0. `positive.json` is
the exact C20 expected passport produced from C14/C18/C19 inputs. The browser
tests select it through the native file input; `negative.json` supplies inert
and malformed variants. The delayed read in one test is simulated solely to
exercise the user-visible cancel action. The other cases use actual browser
file selection and rendering. No fixture is uploaded or executed.

The viewer renders claims but does not validate the passport digest, bundle
files, signer trust or durable revision authority. C22-C26 own later checks.
