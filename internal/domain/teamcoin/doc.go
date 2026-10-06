// Package teamcoin keeps the append-only ledger of team ("color") coins.
//
// For each decided match, the members of a color who bet on it vote: when more
// bettors picked the winner than the loser, the color earns a fixed award. The
// ledger stores the result at settlement and corrects it when a bill is voided,
// so team totals can be audited and do not depend on the current state of bets.
package teamcoin
