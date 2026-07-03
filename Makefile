GOA ?= goa
DESIGN_PKG := github.com/eclipse-xfsc/statuslist-service/design

.PHONY: goa-gen goa-example goa-clean

goa-gen:
	$(GOA) gen $(DESIGN_PKG)

goa-example:
	$(GOA) example $(DESIGN_PKG)

goa-clean:
	rm -rf gen cmd/statuslist

goa-regenerate: goa-clean goa-gen