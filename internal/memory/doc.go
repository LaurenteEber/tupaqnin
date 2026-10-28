// Package memory implements the agent working state: per-task
// bootstrap, the delivered record, caps and the context-usage
// threshold (block 2, specification §5). The working state is
// computed from objects and history at each end of step and cached.
package memory
