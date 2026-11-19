# Local speech programs and models

Studio downloads these only when the user approves the download, into `tools/` in its data folder.
The installer ships none of them. The versions, URLs and SHA-256 pins are in `tool.go`.

- Transcription: NVIDIA NeMo-Speech.cpp 0.3.0, Apache License 2.0, running NVIDIA Parakeet TDT 0.6B v3
  (`nvidia/parakeet-tdt-0.6b-v3`, the 8-bit GGUF file NVIDIA publishes there). The model is by NVIDIA and
  is licensed under CC-BY-4.0 (https://creativecommons.org/licenses/by/4.0/). Studio uses it unchanged.
