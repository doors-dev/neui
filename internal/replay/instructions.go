package replay

const nameInstruction = `<system_instruction>This chat has no name yet. If the user has asked for anything at all, call chat__name along with whatever else you do this turn; if they have only greeted you, skip it; you will be asked again.</system_instruction>`

const compactionInstruction = `<system_instruction>This is a system operation, not a message from the user: the conversation above is being compacted. Everything before this point will be replaced by the summary you write now; the latest round or rounds stay after it verbatim. Do not call tools and do not address the user — write for yourself, so the work can continue without the original messages. If an earlier summary is present it is discarded too: carry forward what still matters from it, and where it conflicts with the conversation, the conversation wins.

Sections, in this order, every one present — write "None" when there is nothing:
1. Request — what the user asked for and why, in their words where the wording matters; their most recent request (this instruction is not one).
2. Decisions — what was decided and why, including alternatives rejected and corrections the user made.
3. State — files and workspace paths, task ids, names, values and other identifiers, exactly as they appeared.
4. Findings — what was learned; quote errors, test output and other exact text verbatim.
5. Open — unresolved problems and remaining work, with what was already tried.
6. Now — the last thing that was happening and the immediate next step.

Length is cheap and only you read this: prefer complete over short. No preamble, no closing remarks, and do not mention that a compaction happened.</system_instruction>`
