# Measuring the sheet-music reader

These scripts score what the reader (`omr/`) gets out of a scanned PDF against a
trusted digital score, so changes can be measured instead of guessed.

1. **Reference notes.** `ly2notes.py` turns a LilyPond file into a note list
   (pitch, onset, hand). It handles what the Mutopia Chopin sources use: relative
   octaves, English or Dutch note names, chords, voices, triplets, grace notes,
   ties, and music variables. The Chopin Op. 9 sources are in the Mutopia
   project (`ftp/ChopinFF/O9/` in github.com/MutopiaProject/MutopiaProject,
   CC BY-SA 3.0):

       python3 ly2notes.py chopin_nocturne_op9_n2.ly ref2.json

2. **Reader output.** `go run ./tools/sheet2song -o out.json score.pdf`
   (add `-quick` for the one-pass read).

3. **Score.** Notes of each hand are ordered by time and aligned
   (Needleman–Wunsch). Differences are counted as wrong pitch (split into
   accidental, step, octave, other), missed and extra notes:

       python3 score_omr.py out.json 1 ref2.json            # piece index 1 = No. 2
       python3 score_omr.py out.json 1 ref2.json --until=120 # bars 1–20 only
       python3 score_omr.py out.json 1 ref2.json --detail    # list every difference

The reference converter is not a full LilyPond implementation; check that its
first bars look right before trusting a score (see the notes in `NOTES.md`).
