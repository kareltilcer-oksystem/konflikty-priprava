-- A note of bare whitespace says nothing (PRD FR-M8).

-- Writes now store such a note as the empty string, so every reader tests for
-- '' alone. This clears the ones written before that rule. SQLite's one-argument
-- trim() strips spaces only, hence the explicit set: space, tab, LF, VT, FF, CR.
UPDATE meeting_items SET prep_note = ''
 WHERE prep_note <> '' AND trim(prep_note, ' ' || char(9, 10, 11, 12, 13)) = '';
UPDATE meeting_items SET action_note = ''
 WHERE action_note <> '' AND trim(action_note, ' ' || char(9, 10, 11, 12, 13)) = '';
UPDATE meetings SET note = ''
 WHERE note <> '' AND trim(note, ' ' || char(9, 10, 11, 12, 13)) = '';
