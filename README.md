<div align="center">

<!-- Banner & Animasi Typing SVG -->
<a href="https://github.com/sr464a6/ngodink">
  <img src="https://readme-typing-svg.demolab.com?font=Plus+Jakarta+Sans&weight=800&size=40&duration=3500&pause=1000&color=6366F1&center=true&vCenter=true&width=700&lines=⚡+WELCOME+TO+KODEY+⚡;The+Next-Gen+Interactive+Coding+Platform;Powered+by+Pure+Go+%2B+AI+Tutor" alt="Typing SVG" />
</a>

<p align="center">
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://anthropic.com"><img src="https://img.shields.io/badge/AI-Anthropic_Claude-D97757?style=for-the-badge&logo=anthropic&logoColor=white" alt="Anthropic"></a>
  <a href="https://midtrans.com"><img src="https://img.shields.io/badge/Payment-Midtrans_QRIS-0050FF?style=for-the-badge&logo=minutemailer&logoColor=white" alt="Midtrans"></a>
  <a href="https://render.com"><img src="https://img.shields.io/badge/Deploy-Render_Docker-46E3B7?style=for-the-badge&logo=render&logoColor=black" alt="Render"></a>
</p>

</div>

---

<details open>
<summary><b>🌌 1. Overview: What is Kodey? (Click to collapse/expand)</b></summary>
<br>

**Kodey** is a blazing-fast, ultra-lightweight interactive programming learning platform equipped with an intelligent AI Tutor. Built completely using Go's standard library with **zero external framework dependencies**, it delivers maximum performance, instant startup times, and seamless deployment anywhere.

</details>

---

<details>
<summary><b>🚀 2. Tech Stack & Architecture</b></summary>
<br>

Kodey is engineered using modern, robust, and minimalist technologies:

| Component | Technology | Role & Function |
| :--- | :--- | :--- |
| **Backend Core** | `Go (Golang) 1.22+` | Handles HTTP routing, business logic, templates, and concurrency with zero external dependencies. |
| **Database** | `Embedded JSON (data.json)` | Lightweight persistence layer for user progress, profiles, and state management. |
| **Content Engine** | `Localized JSON (content.json)` | Dynamic multilingual learning materials, lessons, and modules. |
| **AI Integration** | `Anthropic Claude API` | Acts as an intelligent personal AI Tutor to explain code and answer user questions. |
| **Payment Gateway** | `Midtrans QRIS` | Automated instant QRIS payment processing for premium subscriptions. |
| **Authentication** | `OAuth2 (Google & Apple)` | Secure social sign-in protocols (with fallback Dev-OTP mode). |

</details>

---

<details>
<summary><b>✨ 3. Key Features & User Guide</b></summary>
<br>

### 🎯 What Can Users Do on Kodey?
1. **Learn Interactively:** Read bite-sized programming lessons structured across multiple languages and tracks.
2. **Take Quizzes & Challenges:** Test newly acquired coding knowledge through integrated chapter quizzes.
3. **Chat with AI Tutor:** Ask questions in real-time when stuck on a concept and receive tailored explanations from Claude.
4. **Unlock Premium Content:** Subscribe via QRIS to access advanced modules and deep-dive tutorials.

### 🛠️ Core Capabilities
* **🧠 AI-Powered Tutor:** Integrated directly into the reading and learning workflow to explain complex code snippets and clear roadblocks.
* **⚡ Zero Dependency Core:** Pure Go standard library architecture ensuring extreme speed and tiny resource footprints.
* **🌐 Multilingual Content Hub:** Easily add new programming languages or localized texts directly via `content.json`.
* **💳 Automated QRIS Payments:** Instant subscription activation via Midtrans with fallback manual simulation buttons for testing.
* **🛡️ Flexible Auth & Guest Mode:** Supports Google OAuth, Apple Sign-In, Email OTP, or instant `BYPASS_LOGIN=1` guest entry.
* **🎯 Built-in Ad System:** Dynamic monetization support via `ADS_HTML` (displayed exclusively on reading pages, hidden during quizzes).

</details>

---

<details open>
<summary><b>🛠️ 4. Installation & Access Local Server</b></summary>
<br>

Make sure you have **Go 1.22+** installed on your system.

### Step 1: Clone & Run Locally
```bash
# 1. Clone the repository
git clone [https://github.com/sr464a6/ngodink.git](https://github.com/sr464a6/ngodink.git)
cd ngodink

# 2. Run the application using Go
go run .
