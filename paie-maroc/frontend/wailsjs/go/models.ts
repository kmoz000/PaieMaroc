export namespace attendance {

	export class Device {
	    id: number;
	    name: string;
	    model: string;
	    protocol: string;
	    host: string;
	    port: number;
	    enabled: boolean;
	    lastStatus: string;
	    lastCheckedAt: string;
	    linkedUsers: number;

	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.enabled = source["enabled"];
	        this.lastStatus = source["lastStatus"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.linkedUsers = source["linkedUsers"];
	    }
	}
	export class Holiday {
	    date: string;
	    name: string;
	    kind: string;
	    paid: boolean;
	    premiumBasisPoints: number;
	    status: string;
	    source: string;

	    static createFrom(source: any = {}) {
	        return new Holiday(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.paid = source["paid"];
	        this.premiumBasisPoints = source["premiumBasisPoints"];
	        this.status = source["status"];
	        this.source = source["source"];
	    }
	}
	export class ProbeResult {
	    reachable: boolean;
	    latencyMs: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reachable = source["reachable"];
	        this.latencyMs = source["latencyMs"];
	        this.message = source["message"];
	    }
	}
	export class Profile {
	    model: string;
	    protocols: string[];
	    authentication: string[];
	    faceCapacity: number;
	    fingerCapacity: number;
	    maxEvents: number;
	    notes: string;

	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.protocols = source["protocols"];
	        this.authentication = source["authentication"];
	        this.faceCapacity = source["faceCapacity"];
	        this.fingerCapacity = source["fingerCapacity"];
	        this.maxEvents = source["maxEvents"];
	        this.notes = source["notes"];
	    }
	}
	export class Punch {
	    id: number;
	    employeeId: string;
	    employeeName: string;
	    deviceId: number;
	    deviceName: string;
	    deviceUserId: string;
	    occurredAt: string;
	    direction: string;
	    verification: string;
	    source: string;
	    note: string;
	    edited: boolean;
	    updatedAt: string;
	    unlinked: boolean;

	    static createFrom(source: any = {}) {
	        return new Punch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.employeeId = source["employeeId"];
	        this.employeeName = source["employeeName"];
	        this.deviceId = source["deviceId"];
	        this.deviceName = source["deviceName"];
	        this.deviceUserId = source["deviceUserId"];
	        this.occurredAt = source["occurredAt"];
	        this.direction = source["direction"];
	        this.verification = source["verification"];
	        this.source = source["source"];
	        this.note = source["note"];
	        this.edited = source["edited"];
	        this.updatedAt = source["updatedAt"];
	        this.unlinked = source["unlinked"];
	    }
	}
	export class PunchChange {
	    id: number;
	    punchId: number;
	    action: string;
	    reason: string;
	    changedBy: string;
	    beforeJson: string;
	    afterJson: string;
	    changedAt: string;

	    static createFrom(source: any = {}) {
	        return new PunchChange(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.punchId = source["punchId"];
	        this.action = source["action"];
	        this.reason = source["reason"];
	        this.changedBy = source["changedBy"];
	        this.beforeJson = source["beforeJson"];
	        this.afterJson = source["afterJson"];
	        this.changedAt = source["changedAt"];
	    }
	}
	export class PunchFilter {
	    from: string;
	    to: string;
	    employeeId: string;
	    deviceId: number;
	    query: string;
	    cursor: number;
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new PunchFilter(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.employeeId = source["employeeId"];
	        this.deviceId = source["deviceId"];
	        this.query = source["query"];
	        this.cursor = source["cursor"];
	        this.limit = source["limit"];
	    }
	}
	export class PunchInput {
	    id: number;
	    employeeId: string;
	    deviceId: number;
	    occurredAt: string;
	    direction: string;
	    verification: string;
	    note: string;
	    reason: string;

	    static createFrom(source: any = {}) {
	        return new PunchInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.employeeId = source["employeeId"];
	        this.deviceId = source["deviceId"];
	        this.occurredAt = source["occurredAt"];
	        this.direction = source["direction"];
	        this.verification = source["verification"];
	        this.note = source["note"];
	        this.reason = source["reason"];
	    }
	}
	export class PunchPage {
	    items: Punch[];
	    nextCursor: number;
	    total: number;
	    unlinked: number;

	    static createFrom(source: any = {}) {
	        return new PunchPage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Punch);
	        this.nextCursor = source["nextCursor"];
	        this.total = source["total"];
	        this.unlinked = source["unlinked"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SyncResult {
	    deviceId: number;
	    read: number;
	    imported: number;
	    ignored: number;
	    unlinked: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.read = source["read"];
	        this.imported = source["imported"];
	        this.ignored = source["ignored"];
	        this.unlinked = source["unlinked"];
	        this.message = source["message"];
	    }
	}
	export class WorkPolicy {
	    sector: string;
	    weeklyMinutes: number;
	    dayStart: string;
	    dayEnd: string;
	    breakMinutes: number;
	    graceMinutes: number;
	    dayPremiumBasisPoints: number;
	    nightPremiumBasisPoints: number;
	    restDayPremiumBasisPoints: number;
	    restNightPremiumBasisPoints: number;
	    rulesVersion: string;

	    static createFrom(source: any = {}) {
	        return new WorkPolicy(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sector = source["sector"];
	        this.weeklyMinutes = source["weeklyMinutes"];
	        this.dayStart = source["dayStart"];
	        this.dayEnd = source["dayEnd"];
	        this.breakMinutes = source["breakMinutes"];
	        this.graceMinutes = source["graceMinutes"];
	        this.dayPremiumBasisPoints = source["dayPremiumBasisPoints"];
	        this.nightPremiumBasisPoints = source["nightPremiumBasisPoints"];
	        this.restDayPremiumBasisPoints = source["restDayPremiumBasisPoints"];
	        this.restNightPremiumBasisPoints = source["restNightPremiumBasisPoints"];
	        this.rulesVersion = source["rulesVersion"];
	    }
	}

}

export namespace auth {

	export class CreateInput {
	    username: string;
	    displayName: string;
	    password: string;
	    role: string;

	    static createFrom(source: any = {}) {
	        return new CreateInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.displayName = source["displayName"];
	        this.password = source["password"];
	        this.role = source["role"];
	    }
	}
	export class User {
	    id: number;
	    username: string;
	    displayName: string;
	    role: string;
	    enabled: boolean;
	    createdAt: string;
	    lastLoginAt: string;

	    static createFrom(source: any = {}) {
	        return new User(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.displayName = source["displayName"];
	        this.role = source["role"];
	        this.enabled = source["enabled"];
	        this.createdAt = source["createdAt"];
	        this.lastLoginAt = source["lastLoginAt"];
	    }
	}

}

export namespace exporter {

	export class Company {
	    name: string;
	    affiliateCnss: string;
	    rib: string;
	    participant: string;

	    static createFrom(source: any = {}) {
	        return new Company(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.affiliateCnss = source["affiliateCnss"];
	        this.rib = source["rib"];
	        this.participant = source["participant"];
	    }
	}
	export class StreamResult {
	    recordCount: number;
	    totalCentimes: number;
	    fileCount: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new StreamResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.fileCount = source["fileCount"];
	        this.contract = source["contract"];
	    }
	}

}

export namespace main {

	export class AttendanceExportInput {
	    filter: attendance.PunchFilter;
	    columns: string[];

	    static createFrom(source: any = {}) {
	        return new AttendanceExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filter = this.convertValues(source["filter"], attendance.PunchFilter);
	        this.columns = source["columns"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AttendanceExportResult {
	    path: string;
	    recordCount: number;

	    static createFrom(source: any = {}) {
	        return new AttendanceExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.recordCount = source["recordCount"];
	    }
	}
	export class AuthState {
	    setupRequired: boolean;
	    authenticated: boolean;
	    user?: auth.User;

	    static createFrom(source: any = {}) {
	        return new AuthState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setupRequired = source["setupRequired"];
	        this.authenticated = source["authenticated"];
	        this.user = this.convertValues(source["user"], auth.User);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CNSSResult {
	    path: string;
	    imported: number;
	    existingCount: number;
	    entrantCount: number;
	    employeeCount: number;
	    grossCentimes: number;
	    cappedCentimes: number;

	    static createFrom(source: any = {}) {
	        return new CNSSResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.imported = source["imported"];
	        this.existingCount = source["existingCount"];
	        this.entrantCount = source["entrantCount"];
	        this.employeeCount = source["employeeCount"];
	        this.grossCentimes = source["grossCentimes"];
	        this.cappedCentimes = source["cappedCentimes"];
	    }
	}
	export class ExportInput {
	    format: string;
	    company: exporter.Company;
	    employees: payroll.Employee[];
	    period: string;
	    executionDate: string;
	    batchNumber: number;

	    static createFrom(source: any = {}) {
	        return new ExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.company = this.convertValues(source["company"], exporter.Company);
	        this.employees = this.convertValues(source["employees"], payroll.Employee);
	        this.period = source["period"];
	        this.executionDate = source["executionDate"];
	        this.batchNumber = source["batchNumber"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExportPreview {
	    name: string;
	    mediaType: string;
	    contentBase64: string;
	    recordCount: number;
	    totalCentimes: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new ExportPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mediaType = source["mediaType"];
	        this.contentBase64 = source["contentBase64"];
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.contract = source["contract"];
	    }
	}
	export class ExportResult {
	    path: string;
	    recordCount: number;
	    totalCentimes: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.contract = source["contract"];
	    }
	}
	export class LargeExportInput {
	    format: string;
	    company: exporter.Company;
	    period: string;
	    executionDate: string;
	    batchNumber: number;

	    static createFrom(source: any = {}) {
	        return new LargeExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.company = this.convertValues(source["company"], exporter.Company);
	        this.period = source["period"];
	        this.executionDate = source["executionDate"];
	        this.batchNumber = source["batchNumber"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace payroll {

	export class Employee {
	    id: string;
	    firstName: string;
	    lastName: string;
	    cin: string;
	    cnss: string;
	    rib: string;
	    daysWorked: number;
	    dependants: number;
	    grossCentimes: number;
	    exemptCentimes: number;
	    advanceCentimes: number;

	    static createFrom(source: any = {}) {
	        return new Employee(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.firstName = source["firstName"];
	        this.lastName = source["lastName"];
	        this.cin = source["cin"];
	        this.cnss = source["cnss"];
	        this.rib = source["rib"];
	        this.daysWorked = source["daysWorked"];
	        this.dependants = source["dependants"];
	        this.grossCentimes = source["grossCentimes"];
	        this.exemptCentimes = source["exemptCentimes"];
	        this.advanceCentimes = source["advanceCentimes"];
	    }
	}
	export class Result {
	    employee: Employee;
	    taxableGross: number;
	    employeeCnss: number;
	    employeeAmo: number;
	    professionalExpense: number;
	    incomeTax: number;
	    netPay: number;
	    employerCnss: number;
	    employerAmo: number;
	    familyAllowance: number;
	    trainingTax: number;
	    employerCost: number;
	    rulesVersion: string;

	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.employee = this.convertValues(source["employee"], Employee);
	        this.taxableGross = source["taxableGross"];
	        this.employeeCnss = source["employeeCnss"];
	        this.employeeAmo = source["employeeAmo"];
	        this.professionalExpense = source["professionalExpense"];
	        this.incomeTax = source["incomeTax"];
	        this.netPay = source["netPay"];
	        this.employerCnss = source["employerCnss"];
	        this.employerAmo = source["employerAmo"];
	        this.familyAllowance = source["familyAllowance"];
	        this.trainingTax = source["trainingTax"];
	        this.employerCost = source["employerCost"];
	        this.rulesVersion = source["rulesVersion"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BatchResult {
	    rows: Result[];
	    grossTotalCentimes: number;
	    netTotalCentimes: number;
	    employeeSocialTotal: number;
	    incomeTaxTotal: number;
	    employerChargesTotal: number;
	    employerCostCentimes: number;
	    rulesVersion: string;

	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], Result);
	        this.grossTotalCentimes = source["grossTotalCentimes"];
	        this.netTotalCentimes = source["netTotalCentimes"];
	        this.employeeSocialTotal = source["employeeSocialTotal"];
	        this.incomeTaxTotal = source["incomeTaxTotal"];
	        this.employerChargesTotal = source["employerChargesTotal"];
	        this.employerCostCentimes = source["employerCostCentimes"];
	        this.rulesVersion = source["rulesVersion"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class ValidationIssue {
	    employeeId: string;
	    field: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ValidationIssue(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.employeeId = source["employeeId"];
	        this.field = source["field"];
	        this.message = source["message"];
	    }
	}

}

export namespace store {

	export class CreatePeriodInput {
	    period: string;
	    copyFrom: string;

	    static createFrom(source: any = {}) {
	        return new CreatePeriodInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.copyFrom = source["copyFrom"];
	    }
	}
	export class ImportStats {
	    imported: number;
	    rejected: number;
	    errors: string[];
	    durationMs: number;

	    static createFrom(source: any = {}) {
	        return new ImportStats(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.rejected = source["rejected"];
	        this.errors = source["errors"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class Page {
	    items: payroll.Employee[];
	    nextCursor: string;
	    total: number;

	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], payroll.Employee);
	        this.nextCursor = source["nextCursor"];
	        this.total = source["total"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Period {
	    period: string;
	    status: string;
	    copiedFrom: string;
	    employeeCount: number;
	    grossCentimes: number;
	    createdAt: string;
	    closedAt: string;

	    static createFrom(source: any = {}) {
	        return new Period(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.status = source["status"];
	        this.copiedFrom = source["copiedFrom"];
	        this.employeeCount = source["employeeCount"];
	        this.grossCentimes = source["grossCentimes"];
	        this.createdAt = source["createdAt"];
	        this.closedAt = source["closedAt"];
	    }
	}

}
