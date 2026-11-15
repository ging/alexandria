# Builds the manual (main.pdf) with whatever TeX engine is on PATH, or inside the texlive Docker image.
# The Docker target installs the packages the "small" scheme lacks before running latexmk.

MAIN  := main
OUT   := alexandria-en-la-practica.pdf
IMAGE := texlive/texlive:latest-small
EXTRA := tcolorbox tikzfill pdfcol titlesec xurl environ trimspaces listingsutf8

.PHONY: all docker clean

all: $(OUT)

$(OUT): $(MAIN).tex $(wildcard chapters/*.tex)
	@if command -v latexmk >/dev/null 2>&1; then \
		latexmk -pdf -interaction=nonstopmode -halt-on-error $(MAIN).tex; \
	elif command -v tectonic >/dev/null 2>&1; then \
		tectonic $(MAIN).tex; \
	elif command -v pdflatex >/dev/null 2>&1; then \
		pdflatex -interaction=nonstopmode $(MAIN).tex && pdflatex -interaction=nonstopmode $(MAIN).tex; \
	else \
		echo "No TeX engine on PATH (latexmk, tectonic, pdflatex). Try: make docker"; exit 1; \
	fi
	cp $(MAIN).pdf $(OUT)

docker:
	docker run --rm -v "$(CURDIR)":/work -w /work $(IMAGE) sh -c \
		"tlmgr update --self >/dev/null && tlmgr install $(EXTRA) >/dev/null && latexmk -pdf -interaction=nonstopmode -halt-on-error $(MAIN).tex"
	cp $(MAIN).pdf $(OUT)

clean:
	rm -f $(MAIN).aux $(MAIN).fdb_latexmk $(MAIN).fls $(MAIN).log $(MAIN).out $(MAIN).toc build.log chapters/*.aux
