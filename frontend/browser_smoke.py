import json
from urllib.parse import urlparse
from playwright.sync_api import expect, sync_playwright

API_ORIGIN = "http://localhost:8080"
APP_ORIGIN = "http://localhost:3000"
SCHOOL_ID = "5d2170a2-b847-4d4f-958d-c1d62828600a"
USER_ID = "e2e01f6c-b5b2-4d0a-9f4f-cfd5e518a7a8"

user = {
    "id": USER_ID,
    "school_id": SCHOOL_ID,
    "name": "陈管理员",
    "subject": "",
    "email": "admin@example.com",
    "role": "admin",
    "status": "active",
}
state = {"initialized": False, "authenticated": False, "requests": {}}

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1440, "height": 1000})
    page_errors = []
    page.on("pageerror", lambda error: page_errors.append(str(error)))

    def handle_api(route, request):
        path = urlparse(request.url).path
        headers = {
            "Access-Control-Allow-Origin": APP_ORIGIN,
            "Access-Control-Allow-Credentials": "true",
            "Access-Control-Allow-Methods": "GET,POST,OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type",
        }
        if request.method == "OPTIONS":
            route.fulfill(status=204, headers=headers)
            return
        status = 200
        payload = None
        if path == "/api/v1/init/status":
            payload = {"initialized": state["initialized"]}
        elif path == "/api/v1/init" and request.method == "POST":
            state["requests"]["init"] = request.post_data_json
            state["initialized"] = True
            status = 201
            payload = {"school_code": "01234567"}
        elif path == "/api/v1/auth/register" and request.method == "POST":
            state["requests"]["register"] = request.post_data_json
            status = 201
            payload = {"user": user}
        elif path == "/api/v1/auth/login" and request.method == "POST":
            state["requests"]["login"] = request.post_data_json
            state["authenticated"] = True
            payload = {"user": user}
        elif path == "/api/v1/auth/logout" and request.method == "POST":
            state["authenticated"] = False
            payload = {"status": "ok"}
        elif path == "/api/v1/auth/me":
            if state["authenticated"]:
                payload = {"user": user}
            else:
                status = 401
                payload = {"error": {"code": "UNAUTHORIZED", "message": "请先登录", "details": []}, "request_id": "req-browser"}
        else:
            status = 404
            payload = {"error": {"code": "NOT_FOUND", "message": "接口不存在", "details": []}, "request_id": "req-browser"}
        route.fulfill(status=status, headers={**headers, "Content-Type": "application/json"}, body="" if payload is None else json.dumps(payload))

    page.route(f"{API_ORIGIN}/api/v1/**", handle_api)
    page.goto(APP_ORIGIN)
    page.wait_for_load_state("networkidle")
    expect(page.get_by_role("heading", name="选择访问方式")).to_be_visible()
    expect(page.get_by_role("link", name="首次使用，初始化学校")).to_be_visible()

    page.goto(f"{APP_ORIGIN}/init")
    page.wait_for_load_state("networkidle")
    page.get_by_label("学校名称").fill("实验中学")
    page.get_by_label("管理员姓名").fill("陈管理员")
    page.get_by_label("管理员邮箱").fill("ADMIN@EXAMPLE.COM")
    page.get_by_label("登录密码").fill("StrongPass123!")
    page.get_by_role("button", name="创建学校与管理员").click()
    expect(page.get_by_text("01234567", exact=True)).to_be_visible()
    expect(page.get_by_role("link", name="使用管理员账号登录")).to_be_visible()
    assert state["requests"]["init"] == {"school_name": "实验中学", "admin_name": "陈管理员", "email": "admin@example.com", "password": "StrongPass123!"}

    page.goto(f"{APP_ORIGIN}/init")
    page.wait_for_url(f"{APP_ORIGIN}/login")

    page.goto(f"{APP_ORIGIN}/register")
    page.get_by_label("学校代码").fill("01234567")
    page.get_by_label("姓名", exact=True).fill("周老师")
    page.get_by_label("学科").fill("数学")
    page.get_by_label("邮箱", exact=True).fill("TEACHER@EXAMPLE.COM")
    page.get_by_label("密码", exact=True).fill("TeacherPass123!")
    page.get_by_role("button", name="创建教师账号").click()
    expect(page.get_by_role("heading", name="教师账号已创建")).to_be_visible()
    assert state["requests"]["register"]["school_code"] == "01234567"
    assert state["requests"]["register"]["email"] == "teacher@example.com"

    page.goto(f"{APP_ORIGIN}/login")
    page.screenshot(path="/tmp/school-exam-login.png", full_page=True)
    page.get_by_label("邮箱", exact=True).fill("ADMIN@EXAMPLE.COM")
    page.get_by_label("密码", exact=True).fill("StrongPass123!")
    page.get_by_role("button", name="登录", exact=True).click()
    page.wait_for_url(f"{APP_ORIGIN}/")
    expect(page.get_by_role("heading", name="你好，陈管理员")).to_be_visible()
    expect(page.get_by_text(SCHOOL_ID, exact=True).first).to_be_visible()
    expect(page.get_by_text("学校管理员", exact=True).first).to_be_visible()
    expect(page.get_by_role("button", name="退出", exact=True)).to_be_visible()
    page.screenshot(path="/tmp/school-exam-home.png", full_page=True)

    page.get_by_role("link", name="查看完整资料").click()
    page.wait_for_url(f"{APP_ORIGIN}/account")
    expect(page.get_by_role("heading", name="个人资料")).to_be_visible()
    expect(page.get_by_text(USER_ID, exact=True)).to_be_visible()
    page.get_by_role("button", name="退出", exact=True).click()
    page.wait_for_url(f"{APP_ORIGIN}/login")

    page.get_by_label("邮箱", exact=True).fill("admin@example.com")
    page.get_by_label("密码", exact=True).fill("StrongPass123!")
    page.get_by_role("button", name="登录", exact=True).click()
    page.wait_for_url(f"{APP_ORIGIN}/")
    page.set_viewport_size({"width": 390, "height": 844})
    page.get_by_role("button", name="打开导航").click()
    expect(page.get_by_role("navigation", name="主导航")).to_be_visible()
    page.screenshot(path="/tmp/school-exam-mobile.png", full_page=True)
    assert page.evaluate("document.documentElement.scrollWidth <= window.innerWidth")
    assert not page_errors, page_errors
    print("浏览器验收通过：初始化、状态跳转、注册、登录、真实用户首页、资料、退出与移动导航")
    browser.close()
