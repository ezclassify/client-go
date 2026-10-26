This directory contains sample text files used by the go-classify demo.
Add `.txt` or `.md` files here with marking strings to test the scanner.

The scanner detects banner-style markings: a marking counts only when it
sits on a line of its own, the way a classification banner does. Markings
buried inside a sentence are not matches.

The markings below come from the `cedar-valley-protection` Context.
Example contents for a scannable file:

```
PUBLIC

The published lunch menu goes home with anyone who asks.

CONFIDENTIAL

Purchasing evaluation notes remain with the authorized review team.

RESTRICTED

Live exam questions stay sealed until the assigned testing session.
```
