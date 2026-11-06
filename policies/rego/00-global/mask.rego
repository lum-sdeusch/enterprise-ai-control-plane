# Decision-log masking. OPA applies this before it writes a decision log entry, so raw tool
# arguments, result text and the detailed findings never reach the log store; the adapter's audit
# record keeps the summaries that compliance needs.
package system.log

mask contains "/input/arguments"

mask contains "/input/result_text"

mask contains "/input/findings/pii"

mask contains "/input/findings/secrets"
