#!/bin/sh
set -eu

checked_only=0
if [ "${1:-}" = "--checked-only" ]; then
	checked_only=1
	shift
fi
if [ $# -ne 1 ]; then
	echo "usage: capture.sh [--checked-only] <directory>" >&2
	exit 2
fi

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
mkdir -p "$1"
out=$(cd "$1" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

(cd "$repo" && env -u GOROOT go build -o "$work/bin/app" ./examples/app && env -u GOROOT go build -o "$work/bin/hello" ./examples/hello)
PATH="$work/bin:$PATH"
export PATH
unset NO_COLOR FORCE_COLOR APP_NARRATION HELLO_NARRATION 2>/dev/null || true
cd "$work"
exec </dev/null
esc=$(printf '\033')

run() {
	set +e
	"$@" >"$work/stdout" 2>"$work/stderr"
	code=$?
	set -e
}

ensure_newline() {
	if [ -s "$1" ] && [ "$(tail -c 1 "$1" | od -An -c | tr -d ' ')" != '\n' ]; then
		echo >>"$1"
	fi
}

write_plain() {
	file=$1
	cp "$work/stderr" "$file"
	if [ -s "$work/stdout" ]; then
		ensure_newline "$file"
		echo '---stdout---' >>"$file"
		cat "$work/stdout" >>"$file"
	fi
	if [ "$code" -ne 0 ]; then
		ensure_newline "$file"
		echo "---exit $code---" >>"$file"
	fi
}

plain() {
	name=$1
	shift
	run "$@"
	write_plain "$out/$name.txt"
}

json() {
	name=$1
	shift
	run "$@"
	if [ -s "$work/stdout" ]; then
		echo "capture.sh: $name.jsonl wrote to stdout" >&2
		exit 1
	fi
	cp "$work/stderr" "$out/$name.jsonl"
}

show_escapes() {
	LC_ALL=C tr -d '\r' <"$1" | LC_ALL=C sed "s/$esc/\\\\x1b/g" >"$1.shown"
	mv "$1.shown" "$1"
}

plain install-ok app install jq@1.8.3 --color never
json install-ok app install jq@1.8.3 --narration json
plain install-fail app install zlib@9.9 --color never
plain install-fail-quiet app install zlib@9.9 --quiet --color never
json install-fail-quiet app install zlib@9.9 --quiet --narration json
plain install-corrupt app install corrupt@1.0 --color never
plain install-nosuch app install nosuch --color never
plain update app update --color never
plain update-verbose app update --verbose --color never
plain update-quiet app update --quiet --color never
plain update-color app update --verbose --color always
show_escapes "$out/update-color.txt"
plain check-explicit app check tool broken jq --color never
plain check-sweep-verbose app check --verbose --color never
plain build-broken app build broken@1.0 --color never
plain build-slow app build slow@1.0 --color never
json build-slow app build slow@1.0 --narration json
plain publish app publish registry.example/cat --color never
plain publish-dry-run app publish registry.example/cat --dry-run --color never
printf n | plain clean-prompt app clean --color never
plain clean-yes app clean --yes --color never
plain clean-eof app clean --color never
plain status app status --color never
plain status-json app status --output json --color never
plain info app info jq --color never
plain search app search j --color never
plain search-json app search j --output json --color never
plain search-none app search x --color never
plain env app env --color never
plain run-passthrough app run --color never -- sh -c 'echo out; echo err >&2; exit 3'
plain bad-color app --color bogus status
plain bad-narration app --narration xml status --color never
plain bad-env-narration env APP_NARRATION=xml app status --color never
json bad-env-narration env APP_NARRATION=json app --color bogus status
plain help app --help
plain help-install app install --help
plain sync app sync --color never
plain hello-greet hello greet world --color never
json hello-greet hello greet world --narration json
plain hello-error hello greet nobody --color never

if [ "$checked_only" -eq 1 ]; then
	exit 0
fi

mkdir -p "$out/unchecked"

signal() {
	sig=$1
	shift
	set +e
	"$@" >"$work/stdout" 2>"$work/stderr" &
	pid=$!
	sleep 0.3
	kill -"$sig" "$pid"
	wait "$pid"
	code=$?
	set -e
}

signal INT app build slow@1.0 --narration json
cp "$work/stderr" "$out/unchecked/sigint.jsonl"
signal TERM app build slow@1.0 --color never
write_plain "$out/unchecked/sigterm.txt"

for cols in 80 24; do
	run python3 -I "$here/pty.py" "$cols" -- app build slow@1.0 --color always
	cp "$work/stdout" "$work/frames"
	show_escapes "$work/frames"
	cp "$work/frames" "$out/unchecked/live-$cols.txt"
done
