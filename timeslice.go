/* ibstockcli - A command line program to interact with the IB TWS API using the gofinance/ib library
 *
 * Copyright (C) 2015 Ellery D'Souza <edsouza99@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package main

//------------------------------------------------------------------------------
// Define us a type so we can sort it
type TimeSlice []*ExecutionInfo

// Forward request for length
func (p TimeSlice) Len() int {
	return len(p)
}

// Define compare
func (p TimeSlice) Less(i, j int) bool {
	if p[i].ExecutionData.Exec.Time == p[j].ExecutionData.Exec.Time {
		return p[i].ExecutionData.Exec.CumQty < p[j].ExecutionData.Exec.CumQty
	}
	return p[i].ExecutionData.Exec.Time.Before(p[j].ExecutionData.Exec.Time)
}

// Define swap over an array
func (p TimeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

//------------------------------------------------------------------------------
