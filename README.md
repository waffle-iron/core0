# jsagent
new jump scale agent written in golang

```go
    //some command examples

    cmd := map[string]interface{} {
        "id": "job-id",
        "gid": 1,
        "nid": 10,
        "name": "get_msgs",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
        },
        "data": `{
            "idfrom": 0,
            "idto": 100,
            "timefrom": 100000,
            "timeto": 200000,
            "levels": "3-5"
        }`,
    }

    mem := map[string]interface{} {
        "id": "asdfasdg",
        "gid": 1,
        "nid": 10,
        "name": "get_mem_info",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
        },
    }

    restart := map[string]interface{} {
        "id": "asdfasdg",
        "gid": 1,
        "nid": 10,
        "name": "restart",
        "args": map[string]interface{} {
            "loglevels": []int{1, 2, 3},
            "loglevels_db": []int{3},
            "max_time": 20,
        },
    }

    jscmd := map[string]interface{} {
        "id": "JS-job-id",
        "gid": 1,
        "nid": 10,
        "name": "execute_js_py",
        "args": map[string]interface{} {
            "name": "test.py",
            "loglevels": []int{3},
            //"loglevels_db": []int{3},
            "max_time": 5,
            "recurring_period": 4,
            "max_restart": 2,
        },
        "data": "",
    }

    jscmd2 := map[string]interface{} {
        "id": "recurring",
        "gid": 1,
        "nid": 10,
        "name": "execute_js_py",
        "args": map[string]interface{} {
            "name": "recurring.py",
            "loglevels": []int{3},
            "loglevels_db": []int{3},
            "max_time": 5,
            "recurring_period": 2,
        },
        "data": "",
    }

    killall := map[string]interface{} {
        "id": "kill",
        "gid": 1,
        "nid": 10,
        "name": "killall",
        "args": map[string]interface{} {

        },
    }
```
