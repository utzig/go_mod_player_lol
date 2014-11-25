mod player in go LOL
====================

This is WIP. Currently being done to learn Go.

Module file format: http://www.aes.id.au/modformat.html

DONE
====

* Reading a MOD file seems to work.
* Patterns are not yet intepreted.

TODO
====

1. Samples must be correctly synchronized with
   the soundcard sample rate
2. Mix samples to send columns 1/4 to soundcard left
   and columns 2/3 to soundcard right channel.
3. Interpret and play patterns. At least one at first.
4. Apply effects (MAYBE SOMEDAY).
5. Fix noob bit shifting code to use encoding/binary.
