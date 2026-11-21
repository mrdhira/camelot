Feature: Dhira runs his agents' work from the cockpit
  Dhira only uses the web UI: he sees what waits for him, moves cards,
  creates them, reads documents and sets their review status. Agents use
  the CLI and MCP; the fixture's cards were created through the CLI.

  Scenario: The cockpit puts what waits for Dhira first
    Given Dhira opens "/"
    Then he sees the heading "Waiting for your decision"
    And he sees "Approve the fixture?"
    And he sees the document "PRD900" waiting for review
    And the stat "Blocked" shows "1"

  Scenario: Marking a decision clears the question
    Given Dhira opens "/tasks/FIX-011"
    When he presses "Mark as decided"
    Then the card no longer waits for his decision
    And the history of "FIX-011" shows "waiting: Approve the fixture? → —" by "dhira"

  Scenario: Dragging a card to another column moves it
    Given Dhira opens "/board"
    When he drags card "FIX-010" to the column "plan"
    Then card "FIX-010" is in the column "plan"
    And the history of "FIX-010" shows "moved TODO → Plan in Progress" by "dhira"

  Scenario: A card moves with the keyboard from its page
    Given Dhira opens "/tasks/FIX-013"
    When he chooses "QA in Progress" as the new status and presses "Move"
    Then the card shows the status "QA in Progress"

  Scenario: A new card takes the next number of its prefix
    Given Dhira opens "/tasks/new"
    When he fills "Prefix for the next number" with "FIX" and "Title" with "Made in the browser" and presses "Save"
    Then he is on the page of card "FIX-014"
    And the history of "FIX-014" shows "created in Backlog" by "dhira"

  Scenario: A refused card comes back with the reason
    Given Dhira opens "/tasks/new"
    When he fills "Prefix for the next number" with "FIX" and "Title" with "Bad dependency" and "Depends on" with "NOPE-1" and presses "Save"
    Then he sees "dependency NOPE-1 does not exist"

  Scenario: A document shows its chain and takes a review status
    Given Dhira opens "/docs/PRD900"
    Then he sees the chain "Serves" with "BRD900" and "OKR-2026Q4"
    And he sees the chain "Served by" with "TRD900"
    When he sets the review status to "Approved"
    Then the document shows the status "Approved"

  Scenario: A card's plan renders from its folder
    Given Dhira opens "/tasks/FIX-011/files/PLAN.md"
    Then he sees the heading "FIX — Plan"
    And he sees a table with "go-engineer"

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
