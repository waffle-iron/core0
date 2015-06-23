package utils


func Expand(litral string, min int, max int) []int {
    return []int{1, 2, 3}
}

func In(l []int, x int) bool {
    for i := 0; i < len(l); i++ {
        if l[i] == x {
            return true
        }
    }

    return false
}
