package form

// Desc describes the form control to the agent, for Library.Control.
const Desc = "Form the user fills and submits. A submit gives {\"data\": object, \"submit\": string}, data holding each filled field's value by name. Submit buttons work only once every rule in params passes, validate functions included, so data needs no checks: a field with min 1 and max 1 holds exactly one item, read [0], and only optional fields can be missing. Put actions that must always work, such as Reject or Skip, on cancel or in a separate control. The form keeps its entries after a submit: to have the user fix something only the script can check, show why and call ui.draw() again."
