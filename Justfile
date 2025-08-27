# Simple Justfile for running tests and generating coverage reports
# Usage examples:
#   just test              # run all unit tests
#   just test-cover        # run tests with coverage summary to stdout
#   just test-cover-html   # run tests with coverage and produce coverage.html
#   just clean-coverage    # remove coverage artifacts

# Default recipe
_default: test

# Run all tests
@test:
	go test ./...

# Run tests with coverage profile and function summary
@test-cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

@cover-check:
	go test ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...
	go-test-coverage --config=./.testcoverage.yml

# Generate HTML coverage report (also prints function summary)
@test-cover-html: test-cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report written to coverage.html"

# Clean coverage artifacts
@clean-coverage:
	rm -f coverage.out cover.out coverage.html || true

# Helper to validate mode and return canonical name
_validate_mode mode:
	#!/usr/bin/env bash
	case "{{mode}}" in
		ep|episodes) echo "episodes" ;;
		mv|movies)   echo "movies"   ;;
		se|seasons)  echo "seasons"  ;;
		sw|shows)    echo "shows"    ;;
		config)      echo "config"   ;;
		undo)        echo "undo"     ;;
		*) echo "Invalid mode: {{mode}}" >&2; echo "Allowed: ep|episodes mv|movies se|seasons sw|shows config undo" >&2; exit 1 ;;
	esac


# Helper to publish gif to vhs.charm.sh and return the URL
_publish_gif gif_path:
	@vhs publish {{gif_path}}

# Run an interactive demo dataset for a given media mode.
# Usage examples:
#   just demo ep       # episodes demo
#   just demo mv       # movies demo
#   just demo se       # seasons demo
#   just demo sw       # shows demo
#   just demo config   # config demo
#   just demo undo     # undo demo
# Synonyms accepted:
#   ep|episodes  mv|movies  se|seasons  sw|shows  config  undo
# Steps performed:
# 1. go install . (ensure latest binary in PATH)
# 2. Run matching demo/make.sh to generate demo data under demo/data
# 3. Run title-tidy <canonical-mode> inside the generated dataset directory
# 4. After the command exits, remove the generated demo/data directory
demo target:
	#!/usr/bin/env bash
	set -euo pipefail
	mode=$(just _validate_mode {{target}})
	echo "Installing latest binary (go install .)"
	go install .
	demo_dir="$(pwd)/demo/$mode"
	script_path="${demo_dir}/make.sh"
	if [ ! -x "$script_path" ]; then
		echo "Script not found or not executable: $script_path" >&2
		exit 1
	fi
	echo "Generating demo data via $script_path"
	"$script_path"
	data_dir="${demo_dir}/data"
	if [ ! -d "$data_dir" ]; then
		echo "Expected data directory missing: $data_dir" >&2
		exit 1
	fi
	echo "Entering demo dataset directory: $data_dir"
	( cd "$data_dir" && echo "Running: title-tidy $mode" && title-tidy "$mode" --no-nfo --no-img)
	echo "Cleaning up demo dataset: $data_dir"
	rm -rf "$data_dir"
	echo "Demo $mode complete and cleaned."

# Generate a gif for a specific demo mode
# Usage: just create-gif episodes
# Usage: just create-gif config
# Usage: just create-gif undo
create-gif mode:
	#!/usr/bin/env bash
	set -euo pipefail
	target=$(just _validate_mode {{mode}})
	echo "Installing latest binary (go install .)"
	go install .
	demo_dir="$(pwd)/demo/$target"
	cd "$demo_dir"
	echo "Generating gif for $target demo..."
	if [ "$target" = "config" ]; then
		./backup-config.sh
		vhs demo.tape
		./restore-config.sh
	else
		vhs demo.tape
	fi
	echo "Created $demo_dir/demo.gif"

# Generate, publish and update README for a specific demo
# Usage: just update-gif episodes
# Usage: just update-gif config
# Usage: just update-gif undo
update-gif mode:
	#!/usr/bin/env bash
	set -euo pipefail
	target=$(just _validate_mode {{mode}})
	echo "Installing latest binary (go install .)"
	go install .
	demo_dir="$(pwd)/demo/$target"
	cd "$demo_dir"
	echo "Generating gif for $target demo..."
	if [ "$target" = "config" ]; then
		./backup-config.sh
		vhs demo.tape
		./restore-config.sh
	else
		vhs demo.tape
	fi
	echo "Publishing gif to vhs.charm.sh..."
	gif_url=$(vhs publish demo.gif)
	rm -f demo.gif
	echo "Published to: $gif_url"
	cd ../..
	echo "Updating README.md with new gif URL..."
	# Find the section header for this mode and update the gif URL on the next line
	case "$target" in
		episodes) section_header="### Episodes" ;;
		movies)   section_header="### Movies"   ;;
		seasons)  section_header="### Seasons"  ;;
		shows)    section_header="### Shows"    ;;
		config)   section_header="### Config"   ;;
		undo)     section_header="### Undo"     ;;
	esac
	# Update the gif URL in the line following the section header
	# This matches the pattern ![text](url) and replaces the URL part
	awk -v header="$section_header" -v new_url="$gif_url" '
		$0 == header { found=1; print; next }
		found && /^!\[.*\]\(.*\)$/ {
			sub(/\(https:\/\/vhs\.charm\.sh\/[^)]*\)/, "(" new_url ")")
			found=0
		}
		{ print }
	' README.md > README.md.tmp && mv README.md.tmp README.md
	echo "README.md updated with new $target demo gif"

# Update all demo gifs in README
@update-all-gifs:
	just update-gif config
	just update-gif episodes
	just update-gif movies
	just update-gif seasons
	just update-gif shows
	just update-gif undo
	echo "All demo gifs updated!"
