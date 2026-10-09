# CloudWatch Agent

JSON that cannot be parsed is a failed configuration, not an empty one.

`GenerateMergedJsonConfigMap` returns the directory walk error. A file such as `a` must not fall through to the built-in default document. That default is what made `config-translator` print `Configuration validation first phase succeeded` and exit 0.

The Linux and Darwin control scripts print the translator output and exit 1 when that command fails. They must not continue into the second validation phase or print `Configuration validation succeeded`. Windows already exits through `CheckCMDResult`.

An empty config directory is unchanged: no JSON files still selects the default document.
