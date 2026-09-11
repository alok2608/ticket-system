"use strict";

// Frontend for the ticket system. Talks only to the existing API:
//   POST /auth/register, POST /auth/login,
//   POST /tickets, GET /tickets, GET /tickets/:id, PATCH /tickets/:id/status

var TOKEN_KEY = "ticket_system_token";
var EMAIL_KEY = "ticket_system_email";

// Statuses a ticket can move to, matching the backend's rules.
var NEXT_STATUS = {
	open: ["in_progress", "closed"],
	in_progress: ["closed"],
	closed: []
};

var STATUS_LABEL = {
	open: "Open",
	in_progress: "In progress",
	closed: "Closed"
};

/* ---------- token storage ---------- */

function getToken() {
	return localStorage.getItem(TOKEN_KEY) || "";
}

function setSession(token, email) {
	localStorage.setItem(TOKEN_KEY, token);
	localStorage.setItem(EMAIL_KEY, email);
}

function clearSession() {
	localStorage.removeItem(TOKEN_KEY);
	localStorage.removeItem(EMAIL_KEY);
}

/* ---------- the one place fetch is configured ---------- */

// apiRequest performs a JSON request and returns the parsed body. It attaches
// the bearer token when one is stored, and throws an Error carrying the API's
// message when the response is not successful.
async function apiRequest(method, path, body) {
	var options = {
		method: method,
		headers: { "Content-Type": "application/json" }
	};

	var token = getToken();
	if (token) {
		options.headers["Authorization"] = "Bearer " + token;
	}
	if (body !== undefined) {
		options.body = JSON.stringify(body);
	}

	var response = await fetch(path, options);
	var text = await response.text();
	var data = text ? JSON.parse(text) : null;

	if (!response.ok) {
		var message = data && data.error ? data.error : "Request failed (" + response.status + ")";
		var error = new Error(message);
		error.status = response.status;
		throw error;
	}
	return data;
}

/* ---------- feedback ---------- */

function showMessage(text, type) {
	var box = document.getElementById("message");
	box.textContent = text;
	box.className = "message message-" + type;
	box.hidden = false;
}

function clearMessage() {
	document.getElementById("message").hidden = true;
}

function setLoading(isLoading) {
	var buttons = document.querySelectorAll("button");
	for (var i = 0; i < buttons.length; i++) {
		buttons[i].disabled = isLoading;
	}
}

/* ---------- views ---------- */

function showView(name) {
	var loggedIn = name === "dashboard";
	document.getElementById("auth-view").hidden = loggedIn;
	document.getElementById("dashboard-view").hidden = !loggedIn;
	document.getElementById("user-bar").hidden = !loggedIn;
	document.getElementById("user-email").textContent = localStorage.getItem(EMAIL_KEY) || "";
}

/* ---------- auth ---------- */

async function registerUser(email, password) {
	var data = await apiRequest("POST", "/auth/register", { email: email, password: password });
	setSession(data.token, email);
	showView("dashboard");
	showMessage("Account created. Welcome, " + email + ".", "success");
	await fetchTickets();
}

async function loginUser(email, password) {
	var data = await apiRequest("POST", "/auth/login", { email: email, password: password });
	setSession(data.token, email);
	showView("dashboard");
	showMessage("Logged in as " + email + ".", "success");
	await fetchTickets();
}

function logoutUser() {
	clearSession();
	document.getElementById("ticket-list").innerHTML = "";
	document.getElementById("detail-card").hidden = true;
	showView("auth");
	showMessage("Logged out.", "success");
}

/* ---------- tickets ---------- */

async function fetchTickets() {
	var tickets = await apiRequest("GET", "/tickets");
	renderTickets(tickets);
}

async function createTicket(title, description) {
	await apiRequest("POST", "/tickets", { title: title, description: description });
	showMessage("Ticket created.", "success");
	await fetchTickets();
}

async function updateTicketStatus(id, status) {
	await apiRequest("PATCH", "/tickets/" + id + "/status", { status: status });
	showMessage("Ticket #" + id + " moved to " + STATUS_LABEL[status] + ".", "success");
	await fetchTickets();
}

async function viewTicket(id) {
	var ticket = await apiRequest("GET", "/tickets/" + id);
	renderTicketDetail(ticket);
}

/* ---------- rendering ---------- */

function statusBadge(status) {
	var badge = document.createElement("span");
	badge.className = "badge badge-" + status;
	badge.textContent = STATUS_LABEL[status] || status;
	return badge;
}

// statusControls builds the dropdown and button used to move a ticket forward.
// A closed ticket has no next status, so it gets a plain note instead.
function statusControls(ticket) {
	var wrapper = document.createElement("div");
	wrapper.className = "ticket-actions";

	var view = document.createElement("button");
	view.type = "button";
	view.className = "button button-secondary button-small";
	view.textContent = "View";
	view.addEventListener("click", function () {
		run(function () {
			return viewTicket(ticket.id);
		});
	});
	wrapper.appendChild(view);

	var options = NEXT_STATUS[ticket.status] || [];
	if (options.length === 0) {
		return wrapper;
	}

	var select = document.createElement("select");
	select.setAttribute("aria-label", "New status for ticket " + ticket.id);
	options.forEach(function (status) {
		var option = document.createElement("option");
		option.value = status;
		option.textContent = STATUS_LABEL[status];
		select.appendChild(option);
	});
	wrapper.appendChild(select);

	var update = document.createElement("button");
	update.type = "button";
	update.className = "button button-small";
	update.textContent = "Update";
	update.addEventListener("click", function () {
		run(function () {
			return updateTicketStatus(ticket.id, select.value);
		});
	});
	wrapper.appendChild(update);

	return wrapper;
}

function renderTickets(tickets) {
	var list = document.getElementById("ticket-list");
	list.innerHTML = "";
	document.getElementById("tickets-empty").hidden = tickets.length > 0;

	tickets.forEach(function (ticket) {
		var item = document.createElement("li");
		item.className = "ticket";

		var title = document.createElement("span");
		title.className = "ticket-title";
		title.textContent = "#" + ticket.id + " " + ticket.title;

		item.appendChild(title);
		item.appendChild(statusBadge(ticket.status));
		item.appendChild(statusControls(ticket));
		list.appendChild(item);
	});
}

function renderTicketDetail(ticket) {
	var body = document.getElementById("detail-body");
	body.innerHTML = "";

	var rows = [
		["ID", String(ticket.id)],
		["Title", ticket.title],
		["Description", ticket.description || "—"],
		["Status", STATUS_LABEL[ticket.status] || ticket.status],
		["Created", ticket.created_at],
		["Updated", ticket.updated_at]
	];

	rows.forEach(function (row) {
		var term = document.createElement("dt");
		term.textContent = row[0];
		var value = document.createElement("dd");
		value.textContent = row[1];
		body.appendChild(term);
		body.appendChild(value);
	});

	document.getElementById("detail-card").hidden = false;
}

/* ---------- event wiring ---------- */

// run executes an async action with loading state and one error path, so every
// handler below stays a single call.
async function run(action) {
	clearMessage();
	setLoading(true);
	try {
		await action();
	} catch (error) {
		if (error.status === 401) {
			clearSession();
			showView("auth");
		}
		showMessage(error.message, "error");
	} finally {
		setLoading(false);
	}
}

function init() {
	document.getElementById("login-form").addEventListener("submit", function (event) {
		event.preventDefault();
		var email = document.getElementById("login-email").value.trim();
		var password = document.getElementById("login-password").value;
		run(function () {
			return loginUser(email, password);
		});
	});

	document.getElementById("register-form").addEventListener("submit", function (event) {
		event.preventDefault();
		var email = document.getElementById("register-email").value.trim();
		var password = document.getElementById("register-password").value;
		run(function () {
			return registerUser(email, password);
		});
	});

	document.getElementById("logout-button").addEventListener("click", logoutUser);

	document.getElementById("ticket-form").addEventListener("submit", function (event) {
		event.preventDefault();
		var title = document.getElementById("ticket-title").value.trim();
		var description = document.getElementById("ticket-description").value.trim();
		run(async function () {
			await createTicket(title, description);
			document.getElementById("ticket-form").reset();
		});
	});

	document.getElementById("refresh-button").addEventListener("click", function () {
		run(fetchTickets);
	});

	document.getElementById("detail-close").addEventListener("click", function () {
		document.getElementById("detail-card").hidden = true;
	});

	if (getToken()) {
		showView("dashboard");
		run(fetchTickets);
	} else {
		showView("auth");
	}
}

init();
