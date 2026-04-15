MAIN = bbtkv3-paper

.PHONY: all clean

all: $(MAIN).pdf

$(MAIN).pdf: $(MAIN).tex references.bib
	latexmk -pdf -bibtex $(MAIN).tex

clean:
	latexmk -C $(MAIN).tex
