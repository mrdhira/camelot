# Design book

How Camelot looks and why. The live version, drawn with the real components in both themes, is the `/design` page (footer link "Design book", or ⌘K "Design book"). Tokens: `services/camelot/src/assets/css/tailwind.css`; meanings and their mapping: `services/camelot/src/app/tone.go`; the component: `Tag` in `src/app/tag.templ`.

## 1. A colour means one thing, everywhere

| Meaning | Colour | Used for | Icon (lucide) |
|---|---|---|---|
| **Needs you** | orange | a question waits for Dhira; a card in Review; a document in review | `circle-alert` |
| **Danger** | red | blocked, a document gap, a data problem; tinted for a risk (older than usual) | `ban` (`clock` for age) |
| **Active** | blue | Plan, Work and QA in progress; feedback waiting for the agent | `loader-circle` (`message-square` for feedback) |
| **Done** | green | Done, Approved | `circle-check` |
| **New** | purple | unread changes (the navigation count) | `sparkles` |
| **Neutral** | grey | Backlog, TODO, Draft, Superseded | `circle-dashed` |

Metadata (type, milestone, agent, document type) uses the outline badge or plain text, never a meaning colour.

## 2. Loudness follows urgency

- **Filled**: act now only: Needs you, Blocked (and the unread count).
- **Tint**: states: stages, document statuses, risks.
- **Outline / plain**: metadata.

## 3. Stages say whose turn it is

Backlog, TODO → neutral · Plan, Work, QA in Progress → active · Review in Progress → needs you (Dhira's review) · Done → done. Board columns show the same colour as a dot beside the name; a tile that needs Dhira (a question, or in Review) has an orange edge; the "Needs you" tag or the column's name says why.

## 4. Numbers

A stat tile is coloured only above zero: orange for Waiting on you and In review, red for Blocked and Stale.

## 5. Contrast (WCAG 1.4.3: 4.5:1 for badge text)

| Tone | Light: text on tint | Dark: text on tint | Filled light / dark |
|---|---|---|---|
| Needs you | `#bc4c00` on `#fff1e5` 4.55 | `#f0883e` on `#31251e` 5.87 | 5.03 / 7.53 |
| Danger | `#d1242f` on `#ffebe9` 4.57 | `#ff7b72` on `#342326` 5.90 | 5.24 / 7.39 |
| Active | `#0969da` on `#ddf4ff` 4.56 | `#58a6ff` on `#192a3d` 5.79 | — |
| Done | `#1a7f37` on `#dafbe1` 4.56 | `#3fb950` on `#152d21` 5.80 | — |
| New | `#8250df` on `#fbefff` 4.54 | `#bc8cff` on `#29263d` 5.82 | 5.05 / 7.31 |
| Neutral | `#59636e` on `#eff2f5` 5.44 | `#9198a1` on `#22272e` 5.14 | — |

Light pairs are GitHub Primer's text-on-muted pairs; dark tints are the tone at 16 % over the dark card (`#0d1218`). The accessibility test (axe, WCAG 2.2 AA) runs on every page in both themes.

## 6. The rules

1. One colour, one meaning, on every page. A new badge picks one of the six meanings or stays neutral.
2. Every meaning badge carries a word and the meaning's icon; colour is never the only signal (WCAG 1.4.1).
3. Filled only for act-now. Tints for states. Outline or plain text for metadata.
4. Text never wears a chart colour; a coloured dot or bar beside the text carries it.
5. Both themes, one layout: a new colour gets a light and a dark step, each ≥ 4.5:1 for text.
6. Charts use the validated chart palette (`--series-1`, Insights), not the meanings.
7. Components as shipped (shadcn-templ v2): `Tag` is the shipped `Badge` with tone classes, never a copy.

## Sources

- Atlassian Lozenge (six meanings; "Don't use color alone to signify an important attribute"): https://atlassian.design/components/lozenge/usage
- Jira workflow status categories (grey, blue, green): https://support.atlassian.com/jira-cloud-administration/docs/what-is-a-workflow-status/
- GitHub Primer colour roles and muted/emphasis backgrounds: https://primer.style/foundations/color/overview
- IBM Carbon status indicators ("at least two of … color, shape, or symbol"): https://carbondesignsystem.com/patterns/status-indicator-pattern/
- WCAG 2.2 1.4.1 and 1.4.3: https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html, https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
