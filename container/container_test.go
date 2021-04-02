/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
 */
package container

import (
	"fmt"
	"testing"
)

func TestLeftInsert(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}

	data = append([]int{7}, data...)
	fmt.Println(data)
}
