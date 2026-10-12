// Package teamcoin keeps the append-only ledger of team ("color") coins.
//
// For each decided match, each member of a color who bet on it casts one vote
// for the side they staked more on across all bills. A tie counts as a wrong
// vote. A color earns a fixed award when more members voted for the winner than
// against it. The ledger stores the result at settlement and corrects it when a
// bill is voided, so team totals can be audited and do not depend on the current
// state of bets.
package teamcoin
