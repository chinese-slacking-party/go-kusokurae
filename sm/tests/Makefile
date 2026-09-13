# Builds and runs the C-side test harnesses. These drive the public API
# directly, so they need no cgo and can run under sanitizers that a Go test
# binary cannot.
#
#	make            build and run every check
#	make report     print the score distribution (600k games, not pass/fail)
#	make lint       compile the library alone under the flags cgo enforces
#	make cxx        prove sm.h is still valid C++
#	make bench      build and run ../benchmarks/dealbench.c
#	make clean

CC     ?= gcc
CXX    ?= g++
STD     = -std=c17
WARN    = -Wall -Wextra
SAN     = -fsanitize=address,undefined
INC     = -I ..
LIB     = ../sm.c
HDRS    = ../sm.h ../sm_internal.h
BUILD   = build

# Correctness checks: exit non-zero on failure, so run them under the
# sanitizers. outcomes is excluded because it plays 600k games to sample the
# score distribution -- ASan would stretch that into minutes, and it exercises
# no code path the others miss.
CHECKS  = rulecheck prngcontract ubcheck nativeprng
REPORTS = outcomes

.PHONY: all check report lint cxx bench clean
all: check

$(BUILD):
	mkdir -p $@

$(CHECKS:%=$(BUILD)/%): $(BUILD)/%: %.c $(LIB) $(HDRS) | $(BUILD)
	$(CC) $(STD) $(WARN) $(SAN) $(INC) -o $@ $< $(LIB)

$(REPORTS:%=$(BUILD)/%): $(BUILD)/%: %.c $(LIB) $(HDRS) | $(BUILD)
	$(CC) $(STD) $(WARN) -O2 $(INC) -o $@ $< $(LIB)

check: $(CHECKS:%=$(BUILD)/%)
	@rc=0; \
	for t in $(CHECKS); do \
		echo "========== $$t =========="; \
		$(BUILD)/$$t || rc=1; \
		echo; \
	done; \
	if [ $$rc -eq 0 ]; then echo "all checks passed"; else echo "FAILED"; fi; \
	exit $$rc

report: $(REPORTS:%=$(BUILD)/%)
	@for t in $(REPORTS); do \
		echo "========== $$t =========="; \
		$(BUILD)/$$t; \
	done

# The same flags sm.go pins in its #cgo CFLAGS line, plus -Wstrict-prototypes.
# The library must stay clean under these; the harnesses above are held to the
# looser $(WARN) because they are not shipped.
lint: | $(BUILD)
	$(CC) $(STD) $(WARN) -Wconversion -Wcast-qual -Wstrict-prototypes \
		$(INC) -c -o $(BUILD)/sm.o $(LIB)

# sm.h has C++ consumers, and has been broken by a C-only construct before.
cxx: | $(BUILD)
	$(CXX) -std=c++17 $(WARN) $(INC) -o $(BUILD)/test_main ../_test_main.cxx $(LIB)
	@$(BUILD)/test_main > /dev/null && echo "sm.h compiles and runs as C++"

bench: | $(BUILD)
	$(CC) $(STD) $(WARN) -O2 $(INC) -o $(BUILD)/dealbench ../benchmarks/dealbench.c $(LIB)
	@$(BUILD)/dealbench

clean:
	rm -rf $(BUILD)
