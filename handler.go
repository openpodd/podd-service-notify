package podd_service_notify

import (
	"fmt"
	"net/http"
	"strings"
	"log"
)

const ThankyouTemplate = `
<style>
body {
    font-family: sans-serif;
    font-size: 20px;
    line-height: 1.5em;
    padding: 5px;
}
</style>
<p>ขอบคุณสำหรับการยืนยันรายงานค่ะ</p>
`

type RefNoCache interface {
	Exists(key string) bool
	Set(key string, value string) error
}

type Callback interface {
	Execute(payload Payload) (string, bool)
}

type Server struct {
	Cipher Cipher
	Cache  RefNoCache
}

// return true when refNo already processed
func ValidateRefNo(cache RefNoCache, refNo string) bool {
	if cache.Exists(refNo) {
		return true
	} else {
		cache.Set(refNo, "1")
		return false
	}
}

func (s Server) ZeroReportHandler(callback Callback) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		urlPart := strings.Split(r.URL.Path, "/")
		payload, err := s.Cipher.DecodePayload(urlPart[len(urlPart) - 1])
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// expire
		if payload.IsExpired() {
			fmt.Println("Payload is expired")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// refno
		if ValidateRefNo(s.Cache, payload.RefNo) {
			fmt.Println("Payload is already processed")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			fmt.Println("Payload is a new one")
		}

		if callback != nil {
			_, success := callback.Execute(payload)
			if ! success {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ขอบคุณสำหรับการรายงานค่ะ"))
	}
}

const verifyForm = `
<style>
body {
    font-family: sans-serif;
    font-size: 20px;
    line-height: 1.5em;
    padding: 5px;
}
.hide {
	display: none;
}
.error { color: #e00; }
</style>

<script>
function validate(form) {
	var r1 = document.getElementById('r1');
	var r2 = document.getElementById('r2');
	var r3 = document.getElementById('r3');
	var r4 = document.getElementById('r4');
	var errorIsVerified = document.getElementById('error-isVerified');
	var errorIsOutbreak = document.getElementById('error-isOutbreak');

	var validated = true;

	if (!r1.checked && !r2.checked) {
		errorIsVerified.setAttribute('class', 'error');
		validated = false;
	}
	else {
		errorIsVerified.setAttribute('class', 'error hide');
	}

	if (!r3.checked && !r4.checked) {
		errorIsOutbreak.setAttribute('class', 'error');
		validated = false;
	}
	else {
		errorIsOutbreak.setAttribute('class', 'error hide');
	}

	return validated;
}
</script>

<form method="POST" type="application/x-www-form-urlencoded" onSubmit="return validate(this);">
<input type="hidden" name="reportId" value="23433">
<p>1. ยืนยันว่าสิ่งที่รายงานเป็นเรื่องจริง</p>
<div style="padding: 10px;border: 1px solid #ccc;background-color: #f5f5f5;">
  <input type="radio" id="r1" name="isVerified" value="1" style="margin-right:10px;line-height:45px;"><label for="r1" style="line-height:45px;">ยืนยัน</label><br/>
  <input type="radio" id="r2" name="isVerified" value="0" style="margin-right:10px;line-height:45px;"><label for="r2" style="line-height:45px;">เป็นการทดสอบ ไม่ใช่รายงานจริง</label>
  <div class="error hide" id="error-isVerified">กรุณาเลือกตัวเลือกด้านบน</div>
</div>

<p>2. สถานะการณ์ตอนนี้ ได้ลุกลามมากขึ้นหรือไม่</p>
<div style="padding: 10px;border: 1px solid #ccc;background-color: #f5f5f5;">
  <input type="radio" id="r3" name="isOutbreak" value="1" style="margin-right:10px;line-height:45px;"><label for="r3" style="line-height:45px;">ลุกลาม</label><br/>
  <input type="radio" id="r4" name="isOutbreak" value="0" style="margin-right:10px;line-height:45px;"><label for="r4" style="line-height:45px;">ยังไม่ลุกลาม</label>
  <div class="error hide" id="error-isOutbreak">กรุณาระบุการระบาด</div>
</div>
<button style="border:none;padding: 10px;color: #fff;margin: 15px 0 0;font-size: 18px;background-color: #1C95EF;">ยืนยันข้อมูล</button>
`

var x = `
<script>
var verifyLink = ;
var r1 = document.getElementById('r1');
var r3 = document.getElementById('r3');

function submit() {
	document.write(document."submit");
	var oReq = new XMLHttpRequest();

	oReq.onreadystatechange = function () {
		if (oReq.readyState === 4 && oReq.status === 200) {
			var wrapper = document.getElementsByClassName("wrapper");
			wrapper[0].innerText = "ขอบคุณสำหรับการรายงานค่ะ";
		}
	};

	var isVerified = r1.checked ? 1 : 0;
	var isOutbreak = r3.checked ? 1 : 0;

	oReq.open("POST", verifyLink);
	oReq.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
	oReq.send("isVerified=" + isVerified + "&isOutbreak=" + isOutbreak);

	return false;
}
</script>
`

func (s Server) VerifyReportHandler(callback Callback) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		urlPart := strings.Split(r.URL.Path, "/")
		payload, err := s.Cipher.DecodePayload(urlPart[len(urlPart) - 1])
		if err != nil {
			fmt.Println("Decode error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// expire
		if payload.IsExpired() {
			fmt.Println("Payload is expired")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)

			if s.Cache.Exists(payload.RefNo) {
				w.Write([]byte(ThankyouTemplate))
			} else {
				w.Write([]byte(verifyForm))
			}
			return
		}

		// refno
		if ValidateRefNo(s.Cache, payload.RefNo) {
			fmt.Println("Payload is already processed")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(ThankyouTemplate))
			return
		} else {
			fmt.Println("Payload is a new one")
		}

		if err := r.ParseForm(); err != nil {
			log.Println("Cannot parse form submit", err);
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payload.Form = r.Form

		println("Before callback")
		if callback != nil {
			message, success := callback.Execute(payload)
			if ! success {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(message))
			}
		}
	}
}
