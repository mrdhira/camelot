Feature: Dhira runs his agents' work from the cockpit
  Dhira only uses the web UI: he sees what waits for him, moves cards,
  creates them, reads documents and sets their review status. Agents use
  the CLI and MCP; the fixture's cards were created through the CLI.

  Scenario: The cockpit opens on what changed, then what waits for Dhira
    Given Dhira opens "/"
    Then the first heading is "9 updates"
    And the updates list "PRD900" first
    And he sees the heading "Waiting for your decision"
    And he sees "Approve the fixture?"
    And he sees the document "PRD900" waiting for review
    And the stat "Blocked" shows "1"

  Scenario: Marking everything seen empties what's new
    Given Dhira opens "/"
    When he presses "Mark all seen"
    Then the first heading is "Nothing new"

  Scenario: Answering a question with one press records the decision for the agent
    Given Dhira opens "/"
    When he answers "Rework the plan" for "FIX-011"
    Then he no longer sees "Approve the fixture?"
    And the history of "FIX-011" shows "decided “Rework the plan” on: Approve the fixture?" by "you"

  Scenario: A free-text answer
    Given Dhira opens "/tasks/FIX-015"
    When he writes the answer "Yes, after the review" for "FIX-015"
    Then the history of "FIX-015" shows "decided “Yes, after the review” on: Ship it?" by "you"

  Scenario: A document in review is approved from the cockpit
    Given Dhira opens "/"
    When he approves the document "PRD900"
    Then Dhira opens "/docs/PRD900"
    And the document shows the status "Approved"

  Scenario: A number opens the cards behind it
    Given Dhira opens "/"
    When he opens the stat "Blocked"
    Then he sees "Showing blocked cards."
    And card "FIX-012" is in the column "backlog"

  Scenario: A card opens in the side panel and moves one stage on
    Given Dhira opens "/board"
    When he opens card "FIX-016" in the panel
    Then the panel shows "Card for the panel"
    When he presses "Plan in Progress →"
    Then card "FIX-016" is in the column "plan"

  Scenario: An agent's change appears without reloading
    Given Dhira opens "/board"
    When the agent "go-engineer" moves "FIX-013" to "wip"
    Then card "FIX-013" appears in the column "wip" without a reload

  Scenario: Typing is never lost to a live refresh
    Given Dhira opens "/tasks/new"
    When he types "Half-written title" into "Title" and clicks away
    And the agent "go-engineer" moves "FIX-012" to "todo"
    Then after the next refresh "Title" still holds "Half-written title"

  Scenario: The palette finds a card by keyboard
    Given Dhira opens "/"
    When he presses Command-K and types "blocked"
    And he presses Enter
    Then he is on the page of card "FIX-012"

  Scenario: Dragging a card to another column moves it
    Given Dhira opens "/board"
    When he drags card "FIX-010" to the column "plan"
    Then card "FIX-010" is in the column "plan"
    And the history of "FIX-010" shows "moved TODO → Plan in Progress" by "you"

  Scenario: A card moves to any stage with the keyboard from its page
    Given Dhira opens "/tasks/FIX-013"
    When he chooses "QA in Progress" as another stage
    Then the card shows the status "QA in Progress"

  Scenario: A new card takes the next number of its prefix
    Given Dhira opens "/tasks/new"
    When he fills "Prefix for the next number" with "FIX" and "Title" with "Made in the browser" and presses "Save"
    Then he is on the page of card "FIX-018"
    And the history of "FIX-018" shows "created in Backlog" by "you"

  Scenario: A refused card comes back with the reason
    Given Dhira opens "/tasks/new"
    When he fills "Prefix for the next number" with "FIX" and "Title" with "Bad dependency" and "Depends on" with "NOPE-1" and presses "Save"
    Then he sees "dependency NOPE-1 does not exist"

  Scenario: A document shows its contents, chain and history, and takes a review status
    Given Dhira opens "/docs/PRD900"
    Then the contents list "Goals" and "Measures"
    And he sees the chain "Serves" with "BRD900" and "OKR-2026Q4"
    And he sees the chain "Served by" with "TRD900"
    When he sets the review status to "Approved"
    Then the document shows the status "Approved"

  Scenario: A card's plan renders from its folder
    Given Dhira opens "/tasks/FIX-011/files/PLAN.md"
    Then he sees the heading "FIX — Plan"
    And he sees a table with "go-engineer"

  Scenario: A comment on a card waits for the agent's reply, then is resolved
    Given Dhira opens "/tasks/FIX-017"
    When he comments "Please split the plan in two."
    Then he sees "Feedback for the agent: no reply yet"
    And on the board, card "FIX-017" shows "Feedback for the agent" and "1 open comment"
    Given Dhira opens "/tasks/FIX-017"
    When the agent "go-engineer" comments "Split into two cards." on "FIX-017"
    Then "Split into two cards." appears without a reload
    And he no longer sees "Feedback for the agent: no reply yet"
    When he resolves the comment "Please split the plan in two."
    Then he sees "resolved by you"
    And he sees "1 open"

  Scenario: A comment on a section of a document
    Given Dhira opens "/docs/PRD900"
    When he comments "Which measure comes first?" about "Measures"
    Then he sees the comment "Which measure comes first?" about "§ Measures"

  Scenario: The theme follows the system until switched, then stays (from tablet width up; a phone follows the system)
    Given Dhira's system is set to dark
    And Dhira opens "/"
    Then the page is dark
    And the theme switch shows "Dark"
    When he flips the theme switch
    Then the page is light
    And after a reload the page is still light
    When he flips the theme switch
    Then the page is dark

  Scenario Outline: Every page is accessible and fits a phone
    Given Dhira opens "<path>" on a 390 px wide screen
    Then the page has no horizontal scroll
    And the page has no detectable accessibility violations

    Examples:
      | path                        |
      | /                           |
      | /board                      |
      | /insights                   |
      | /docs                       |
      | /docs/PRD900                |
      | /tasks/FIX-011              |
      | /tasks/new                  |
      | /check                      |
      | /tasks/FIX-011/files/PLAN.md |

  Scenario Outline: Every page is accessible on a desktop
    Given Dhira opens "<path>" on a 1280 px wide screen
    Then the page has no horizontal scroll
    And the page has no detectable accessibility violations

    Examples:
      | path           |
      | /              |
      | /board         |
      | /insights      |
      | /docs          |
      | /tasks/FIX-011 |

  Scenario Outline: Every page is accessible in the dark theme
    Given Dhira's system is set to dark
    And Dhira opens "<path>" on a <width> px wide screen
    Then the page has no horizontal scroll
    And the page has no detectable accessibility violations

    Examples:
      | path           | width |
      | /              | 1280  |
      | /board         | 1280  |
      | /insights      | 1280  |
      | /docs          | 1280  |
      | /docs/PRD900   | 1280  |
      | /tasks/FIX-011 | 1280  |
      | /              | 390   |
      | /tasks/FIX-011 | 390   |
      | /docs/PRD900   | 390   |
